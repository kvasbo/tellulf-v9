package power

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/spotpris"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Tomorrow's prices come out around 13:00. From then on, until they are
// there, they are asked for at most every tomorrowRecheck.
const (
	tomorrowFrom    = 13
	tomorrowRecheck = 15 * time.Minute
)

// wantedDays are the days whose prices are needed at now: every day of the
// month so far, to price its hours, and tomorrow from 13:00.
func wantedDays(now time.Time) []time.Time {
	now = now.In(tz.Oslo)
	last := tz.StartOfDay(now)
	if now.Hour() >= tomorrowFrom {
		last = last.AddDate(0, 0, 1)
	}
	var out []time.Time
	for d := startOfMonth(now); !d.After(last); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// runPrices fetches the days in wantedDays that it doesn't have, each once.
// A failure stops the round and waits with backoff.
func (p *Power) runPrices(ctx context.Context) {
	failures := 0
	var retryAt, tomorrowAt time.Time
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		now := tz.Now()
		if now.Before(retryAt) {
			return
		}
		p.dropPricesBefore(startOfMonth(now))
		tomorrow := tz.StartOfDay(now).AddDate(0, 0, 1)
		fetched := 0
		for _, day := range wantedDays(now) {
			if p.hasPrices(day) || day.Equal(tomorrow) && now.Before(tomorrowAt) {
				continue
			}
			prices, err := p.cfg.Prices.Day(ctx, day)
			if errors.Is(err, spotpris.ErrNotPublished) && day.Equal(tomorrow) {
				tomorrowAt = now.Add(tomorrowRecheck)
				continue
			}
			if err != nil {
				failures++
				retryAt = now.Add(schedule.Backoff(failures))
				slog.Error("could not fetch spot prices", "day", day.Format(time.DateOnly), "err", err,
					"retry_at", retryAt.Format(time.TimeOnly))
				break
			}
			failures = 0
			p.storePrices(day, prices)
			fetched++
		}
		if fetched > 0 {
			slog.Info("spot prices fetched", "days", fetched)
		}
	})
}

func (p *Power) hasPrices(day time.Time) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.prices[day.Unix()]
	return ok
}

func (p *Power) storePrices(day time.Time, prices []spotpris.Price) {
	points := make([]PricePoint, len(prices))
	for i, pr := range prices {
		points[i] = PricePoint{Start: pr.Start, Total: pr.Total}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prices[day.Unix()] = points
}

func (p *Power) dropPricesBefore(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for day := range p.prices {
		if day < t.Unix() {
			delete(p.prices, day)
		}
	}
}

// spotAt is the spot price for the hour starting at h, or 0 when it isn't
// known. The caller holds the lock.
func (p *Power) spotAt(h time.Time) float64 {
	for _, pp := range p.prices[tz.StartOfDay(h).Unix()] {
		if pp.Start.Unix() == h.Unix() {
			return pp.Total
		}
	}
	return 0
}

// priceNow is the latest price that has started by now: the hour now, or
// when the lists don't have it, the last known price rather than zero. The
// caller holds the lock.
func (p *Power) priceNow(now time.Time) float64 {
	var latest PricePoint
	for _, day := range p.prices {
		for _, pp := range day {
			if !pp.Start.After(now) && pp.Start.After(latest.Start) {
				latest = pp
			}
		}
	}
	return latest.Total
}
