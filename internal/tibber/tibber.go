// Package tibber tracks power usage and cost for the home and the cabin:
// a live websocket feed from each Pulse, daily price lists, and monthly
// consumption totals, combined with the Norgespris rules.
package tibber

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

type Place string

const (
	Home  Place = "home"
	Cabin Place = "cabin"
)

// PowerData is what the display shows for one place.
type PowerData struct {
	Timestamp              time.Time // last live measurement; zero until the feed delivers
	AccumulatedConsumption float64   // kWh since midnight, net of production
	AccumulatedProduction  float64   // kWh produced since midnight
	AccumulatedCost        float64   // kr since midnight
	CurrentPower           float64   // W, negative when producing
	MinPower               float64   // W since midnight
	AveragePower           float64
	MaxPower               float64
	MaxPowerProduction     float64
	CurrentPrice           float64 // spot price incl. VAT, kr/kWh
	EffectivePrice         float64 // what a kWh costs right now under Norgespris
	MonthlyConsumption     float64 // kWh this month
	MonthlyCost            float64 // kr this month
	Cap                    float64 // Norgespris monthly cap, kWh
}

type Config struct {
	Token   string
	HomeID  string
	CabinID string
}

type Tibber struct {
	gql        *graphQL
	homeIDs    map[Place]string
	norgespris Norgespris

	mu     sync.RWMutex
	data   map[Place]*PowerData
	prices map[Place][]PricePoint
	// Monthly totals up to (not including) today, from the consumption API.
	monthBeforeToday     map[Place]float64
	monthCostBeforeToday map[Place]float64
	lastCabinProduction  float64
	// For the power page: this month's days before today and today's hours
	// from the consumption API, and today's hours from the live feed.
	days         map[Place][]Usage
	historyToday map[Place][]Usage
	live         map[Place]*liveHours
}

func New(cfg Config) *Tibber {
	t := &Tibber{
		gql: &graphQL{
			http:     &http.Client{Timeout: 30 * time.Second},
			endpoint: "https://api.tibber.com/v1-beta/gql",
			token:    cfg.Token,
		},
		homeIDs:              map[Place]string{Home: cfg.HomeID, Cabin: cfg.CabinID},
		norgespris:           defaultNorgespris,
		data:                 map[Place]*PowerData{},
		prices:               map[Place][]PricePoint{},
		monthBeforeToday:     map[Place]float64{},
		monthCostBeforeToday: map[Place]float64{},
		days:                 map[Place][]Usage{},
		historyToday:         map[Place][]Usage{},
		live:                 map[Place]*liveHours{},
	}
	for _, p := range []Place{Home, Cabin} {
		t.data[p] = &PowerData{Cap: t.norgespris.Cap(p)}
		t.live[p] = &liveHours{}
	}
	return t
}

// PowerData returns a copy of the latest numbers for p.
func (t *Tibber) PowerData(p Place) PowerData {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return *t.data[p]
}

// Run starts the feeds and update loops, returning when ctx is cancelled.
func (t *Tibber) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i, p := range []Place{Home, Cabin} {
		f := &feed{gql: t.gql, place: p, homeID: t.homeIDs[p], onMessage: t.onMeasurement}
		wg.Go(func() {
			// Stagger the two feeds so they never open connections at once.
			if schedule.Sleep(ctx, time.Duration(i)*5*time.Second) {
				f.run(ctx)
			}
		})
		wg.Go(func() { t.runPrices(ctx, p) })
		wg.Go(func() { t.runMonthly(ctx, p) })
	}
	wg.Wait()
}

// onMeasurement folds one live measurement into the place's numbers.
func (t *Tibber) onMeasurement(p Place, m liveMeasurement) {
	val := func(f *float64) float64 {
		if f == nil {
			return 0
		}
		return *f
	}

	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.live[p].add(now, m.AccumulatedConsumption)
	d := t.data[p]
	d.Timestamp = now
	d.AccumulatedConsumption = m.AccumulatedConsumption - m.AccumulatedProduction
	d.AccumulatedProduction = m.AccumulatedProduction
	d.CurrentPower = m.Power
	d.MaxPower = m.MaxPower
	d.MinPower = m.MinPower
	d.AveragePower = m.AveragePower
	d.MaxPowerProduction = val(m.MaxPowerProduction)

	// Monthly consumption = history before today + today's live total.
	if before, ok := t.monthBeforeToday[p]; ok {
		d.MonthlyConsumption = before + d.AccumulatedConsumption
	}

	// Split today's consumption into the part still under the monthly cap
	// (Norgespris) and the part above it (spot price).
	today := d.AccumulatedConsumption
	beforeToday := max(0, d.MonthlyConsumption-today)
	subsidized := max(0, min(today, d.Cap-beforeToday))
	market := today - subsidized
	d.AccumulatedCost = subsidized*t.norgespris.PriceAt(now) + market*d.CurrentPrice
	d.EffectivePrice = t.norgespris.EffectivePrice(p, now, d.MonthlyConsumption, d.CurrentPrice)

	if before, ok := t.monthCostBeforeToday[p]; ok {
		d.MonthlyCost = before + d.AccumulatedCost
	}

	// The cabin has solar panels: when it reports zero consumption, show the
	// latest production as negative power instead.
	if p == Cabin {
		if m.PowerProduction != nil {
			t.lastCabinProduction = *m.PowerProduction
		}
		if m.Power == 0 {
			d.CurrentPower = -t.lastCabinProduction
		}
	}
}

// --- Prices -------------------------------------------------------------------

const priceSlot = time.Hour // matches priceInfo(resolution: HOURLY)

// Tibber publishes tomorrow's prices around 13:00. From then on, a list
// without tomorrow is fetched again, at most every tomorrowRecheck.
const (
	tomorrowFrom    = 13
	tomorrowRecheck = 15 * time.Minute
)

// runPrices re-derives the current price every minute from the cached price
// list. It fetches a new list when the cache doesn't cover the current hour
// (at midnight, since each fetch includes tomorrow), and in the afternoon
// until the list has tomorrow, for the power page.
func (t *Tibber) runPrices(ctx context.Context, p Place) {
	failures := 0
	var nextAttempt time.Time
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		now := time.Now()
		_, haveNow := t.priceAt(p, now)
		wantTomorrow := t.missingTomorrow(p, now)
		if (!haveNow || wantTomorrow) && !now.Before(nextAttempt) {
			prices, err := t.gql.prices(ctx, t.homeIDs[p])
			if err != nil {
				failures++
				nextAttempt = now.Add(backoff(failures))
				slog.Error("could not fetch prices", "place", p, "err", err, "retry_at", nextAttempt.Format(time.TimeOnly))
			} else {
				failures = 0
				t.mu.Lock()
				t.prices[p] = prices
				t.mu.Unlock()
				slog.Info("prices fetched", "place", p, "slots", len(prices))
				if t.missingTomorrow(p, now) {
					nextAttempt = now.Add(tomorrowRecheck) // not published yet
				}
			}
		}
		t.updatePrice(p, now)
	})
}

// missingTomorrow: it is past the time tomorrow's prices are published, and
// the cache doesn't have them.
func (t *Tibber) missingTomorrow(p Place, now time.Time) bool {
	if now.In(tz.Oslo).Hour() < tomorrowFrom {
		return false
	}
	_, ok := t.priceAt(p, tz.StartOfDay(now).AddDate(0, 0, 1))
	return !ok
}

func (t *Tibber) priceAt(p Place, at time.Time) (float64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, pp := range t.prices[p] {
		if !at.Before(pp.Start) && at.Before(pp.Start.Add(priceSlot)) {
			return pp.Total, true
		}
	}
	return 0, false
}

func (t *Tibber) updatePrice(p Place, now time.Time) {
	spot, ok := t.priceAt(p, now)
	if !ok {
		return // keep the last known price rather than showing zero
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	d := t.data[p]
	d.CurrentPrice = spot
	d.EffectivePrice = t.norgespris.EffectivePrice(p, now, d.MonthlyConsumption, spot)
}

// --- Monthly consumption ----------------------------------------------------------

// runMonthly fetches this month's consumption shortly after start and then
// on every full hour. Consumption isn't real-time, so polling more often is
// pointless.
func (t *Tibber) runMonthly(ctx context.Context, p Place) {
	if !schedule.Sleep(ctx, 2*time.Second) { // let the feeds deliver today's total first
		return
	}
	t.updateMonthly(ctx, p)
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		if tz.Now().Minute() == 0 {
			t.updateMonthly(ctx, p)
		}
	})
}

func (t *Tibber) updateMonthly(ctx context.Context, p Place) {
	now := tz.Now()
	hours := now.Day()*24 + now.Hour() + 3
	nodes, err := t.gql.hourlyConsumption(ctx, t.homeIDs[p], hours)
	if err != nil {
		slog.Error("could not fetch monthly consumption", "place", p, "err", err)
		return
	}
	usage := monthHoursBeforeToday(nodes, now)
	total := 0.0
	for _, h := range usage {
		total += h.Consumption
	}
	cost := t.norgespris.AccumulatedCost(p, usage)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.monthBeforeToday[p] = total
	t.monthCostBeforeToday[p] = cost
	t.days[p] = dayTotals(usage)
	t.historyToday[p] = historyToday(nodes, now)
	d := t.data[p]
	d.MonthlyConsumption = total + d.AccumulatedConsumption
	d.MonthlyCost = cost + d.AccumulatedCost
	slog.Debug("monthly consumption fetched", "place", p, "kwh_before_today", total)
}

// monthHoursBeforeToday picks the hours from the start of the month up to
// (not including) today, in order, with a per-kWh price derived from the
// reported cost.
func monthHoursBeforeToday(nodes []consumptionNode, now time.Time) []hourlyUsage {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
	startOfToday := tz.StartOfDay(now)
	var out []hourlyUsage
	for _, n := range nodes {
		if n.From.Before(startOfMonth) || !n.From.Before(startOfToday) {
			continue
		}
		h := hourlyUsage{Start: n.From}
		if n.Consumption != nil {
			h.Consumption = *n.Consumption
		}
		if h.Consumption > 0 && n.Cost != nil {
			h.Price = *n.Cost / h.Consumption
		}
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b hourlyUsage) int { return a.Start.Compare(b.Start) })
	return out
}
