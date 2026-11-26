// Package power puts together what the screen shows about power at home and
// at the cabin: the hourly consumption (Elvia at home, Tibber at the cabin),
// the cabin's live feed, spot prices from hvakosterstrommen.no and the
// Norgespris rules. It keeps the raw data and works the numbers out when
// they are read.
package power

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/elvia"
	"github.com/kvasbo/tellulf-v9/internal/spotpris"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

type Place string

const (
	Home  Place = "home"
	Cabin Place = "cabin"
)

// PowerData is what the display shows for one place.
type PowerData struct {
	Timestamp              time.Time // last live measurement; zero without a feed
	AccumulatedConsumption float64   // kWh today, net of production
	AccumulatedProduction  float64   // kWh produced today
	AccumulatedCost        float64   // kr today
	// TodayUntil: without live numbers, today's totals are the known hours,
	// up to the end of the last one. Zero with live numbers, or no hours yet.
	TodayUntil         time.Time
	CurrentPower       float64 // W, negative when producing
	MinPower           float64 // W today
	AveragePower       float64
	MaxPower           float64
	MaxPowerProduction float64
	CurrentPrice       float64 // spot price incl. VAT, kr/kWh
	EffectivePrice     float64 // what a kWh costs right now under Norgespris
	MonthlyConsumption float64 // kWh this month
	MonthlyCost        float64 // kr this month
	Cap                float64 // Norgespris monthly cap, kWh
	MonthKnown         bool    // this month's hours have been fetched
}

// staleAfter is how long the last live measurement counts as the power
// right now. A Pulse reports every few seconds, and the feed reconnects after
// five minutes of silence, so this means it is really gone.
const staleAfter = 10 * time.Minute

// IsLive says whether the live numbers can be shown as now and today: a dead
// feed would otherwise leave its last reading up as if it were live, and
// after midnight show the day before's totals as today's.
func (d PowerData) IsLive(now time.Time) bool {
	return now.Sub(d.Timestamp) <= staleAfter && tz.SameDay(d.Timestamp, now)
}

// Usage is the energy used in one hour or one day.
type Usage struct {
	Start       time.Time
	Consumption float64 // kWh
}

// PricePoint is the spot price for one hour.
type PricePoint struct {
	Start time.Time
	Total float64 // kr/kWh incl. VAT
}

// Details is everything the power page shows for one place.
type Details struct {
	PowerData
	Hours      []Usage      // today's hours in order; an hour without data is missing
	Days       []Usage      // this month's days before today
	Prices     []PricePoint // today, and tomorrow once published
	Norgespris float64      // the fixed price now, kr/kWh
}

// Config has the sources; a nil one is left out, and its place stays blank.
type Config struct {
	Elvia  *elvia.Client    // the home's hours
	Tibber *tibber.Client   // the cabin's feed and hours
	Prices *spotpris.Client // spot prices for both
}

type Power struct {
	cfg        Config
	norgespris Norgespris

	mu      sync.RWMutex
	prices  map[int64][]PricePoint // Oslo day start (Unix) → that day's hours
	hours   map[Place][]Usage      // hours from the history, sorted
	hoursAt map[Place]time.Time    // when hours was last fetched
	live    map[Place]PowerData    // the feed's numbers only
	marks   map[Place]*liveHours
	// The cabin's latest solar production, W.
	lastCabinProduction float64
	hanHome             hanHome
}

func New(cfg Config) *Power {
	return &Power{
		cfg:        cfg,
		norgespris: defaultNorgespris,
		prices:     map[int64][]PricePoint{},
		hours:      map[Place][]Usage{},
		hoursAt:    map[Place]time.Time{},
		live:       map[Place]PowerData{},
		marks:      map[Place]*liveHours{Home: {}, Cabin: {}},
	}
}

// Run starts the loops for the sources there are, returning when ctx is
// cancelled.
func (p *Power) Run(ctx context.Context) {
	var wg sync.WaitGroup
	if p.cfg.Prices != nil {
		wg.Go(func() { p.runPrices(ctx) })
	}
	if p.cfg.Elvia != nil {
		wg.Go(func() { p.runElvia(ctx) })
	}
	if p.cfg.Tibber != nil {
		wg.Go(func() { p.cfg.Tibber.RunFeed(ctx, func(m tibber.Measurement) { p.Measure(Cabin, m) }) })
		wg.Go(func() { p.runTibberHours(ctx) })
	}
	wg.Wait()
}

// Measure takes a live measurement for pl.
func (p *Power) Measure(pl Place, m tibber.Measurement) {
	p.measure(pl, m, time.Now())
}

func (p *Power) measure(pl Place, m tibber.Measurement, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.marks[pl].add(now, m.AccumulatedConsumption)
	d := PowerData{
		Timestamp:              now,
		AccumulatedConsumption: m.AccumulatedConsumption - m.AccumulatedProduction,
		AccumulatedProduction:  m.AccumulatedProduction,
		CurrentPower:           m.Power,
		MaxPower:               m.MaxPower,
		MinPower:               m.MinPower,
		AveragePower:           m.AveragePower,
	}
	if m.MaxPowerProduction != nil {
		d.MaxPowerProduction = *m.MaxPowerProduction
	}
	// The cabin has solar panels: when it reports zero consumption, show the
	// latest production as negative power instead.
	if pl == Cabin {
		if m.PowerProduction != nil {
			p.lastCabinProduction = *m.PowerProduction
		}
		if m.Power == 0 {
			d.CurrentPower = -p.lastCabinProduction
		}
	}
	p.live[pl] = d
}

// Data returns the numbers for pl as they stand at now.
func (p *Power) Data(pl Place, now time.Time) PowerData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.derive(pl, now)
}

// Details returns everything the power page shows for pl at now.
func (p *Power) Details(pl Place, now time.Time) Details {
	p.mu.RLock()
	defer p.mu.RUnlock()
	before, today := p.split(pl, now)
	day := tz.StartOfDay(now)
	return Details{
		PowerData:  p.derive(pl, now),
		Hours:      mergeHours(today, p.marks[pl].hours(now), now),
		Days:       dayTotals(before),
		Prices:     slices.Concat(p.prices[day.Unix()], p.prices[day.AddDate(0, 0, 1).Unix()]),
		Norgespris: p.norgespris.PriceAt(now),
	}
}

// split picks this month's hours from the history: those before today, and
// today's.
func (p *Power) split(pl Place, now time.Time) (before, today []Usage) {
	monthStart, dayStart := startOfMonth(now), tz.StartOfDay(now)
	for _, h := range p.hours[pl] {
		switch {
		case h.Start.Before(monthStart):
		case h.Start.Before(dayStart):
			before = append(before, h)
		case tz.SameDay(h.Start, now):
			today = append(today, h)
		}
	}
	return before, today
}

// derive works out pl's numbers at now. The caller holds the lock.
func (p *Power) derive(pl Place, now time.Time) PowerData {
	n := p.norgespris
	d := p.live[pl]
	d.Cap = n.Cap(pl)
	d.MonthKnown = !p.hoursAt[pl].Before(startOfMonth(now))
	d.CurrentPrice = p.priceNow(now)

	before, today := p.split(pl, now)
	used := 0.0
	for _, h := range before {
		used += h.Consumption
	}
	switch {
	case d.IsLive(now) && pl == Home:
		// The HAN reader has only counted since it started, so today is the
		// hours: Elvia's where it has them, the reader's for the rest.
		hours := mergeHours(today, p.marks[pl].hours(now), now)
		d.AccumulatedConsumption = 0
		for _, h := range hours {
			d.AccumulatedConsumption += h.Consumption
		}
		if gone := now.Sub(tz.StartOfDay(now)).Hours(); gone > 0 {
			d.AveragePower = d.AccumulatedConsumption * 1000 / gone
		}
		d.AccumulatedCost = n.AccumulatedCost(pl, hours, p.spotAt, used)
	case d.IsLive(now):
		// Split today's live total into the part still under the monthly cap
		// (Norgespris) and the part above it (spot price now).
		subsidized := max(0, min(d.AccumulatedConsumption, d.Cap-used))
		market := d.AccumulatedConsumption - subsidized
		d.AccumulatedCost = subsidized*n.PriceAt(now) + market*d.CurrentPrice
	default:
		d.AccumulatedConsumption, d.AccumulatedProduction, d.TodayUntil = 0, 0, time.Time{}
		for _, h := range today {
			d.AccumulatedConsumption += h.Consumption
			d.TodayUntil = h.Start.Add(time.Hour)
		}
		d.AccumulatedCost = n.AccumulatedCost(pl, today, p.spotAt, used)
	}
	d.MonthlyConsumption = used + d.AccumulatedConsumption
	d.MonthlyCost = n.AccumulatedCost(pl, before, p.spotAt, 0) + d.AccumulatedCost
	d.EffectivePrice = n.EffectivePrice(pl, now, d.MonthlyConsumption, d.CurrentPrice)
	return d
}

// storeHours replaces pl's hours from the history.
func (p *Power) storeHours(pl Place, hours []Usage, now time.Time) {
	slices.SortFunc(hours, func(a, b Usage) int { return a.Start.Compare(b.Start) })
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hours[pl] = hours
	p.hoursAt[pl] = now
}

func startOfMonth(t time.Time) time.Time {
	t = t.In(tz.Oslo)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, tz.Oslo)
}
