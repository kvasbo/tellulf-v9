package power

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// elviaEvery: Elvia has an hour less than an hour after it ends, so a check
// every quarter keeps the day close to now without asking much.
const elviaEvery = 15 * time.Minute

// elviaPace says when to fetch the home's hours: every elviaEvery, at once
// when a new month starts (until then the month isn't known, and the home
// is blank), and with backoff after a failure.
type elviaPace struct {
	month    time.Time // the month of the last success
	next     time.Time
	failures int
}

func (e *elviaPace) due(now time.Time) bool {
	newMonth := !startOfMonth(now).Equal(e.month)
	return !now.Before(e.next) || newMonth && e.failures == 0
}

func (e *elviaPace) done(now time.Time, err error) {
	if err != nil {
		e.failures++
		e.next = now.Add(schedule.Backoff(e.failures))
		return
	}
	e.month, e.next, e.failures = startOfMonth(now), now.Add(elviaEvery), 0
}

// runElvia fetches the home's hours for the month so far, as elviaPace says.
func (p *Power) runElvia(ctx context.Context) {
	var pace elviaPace
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		now := tz.Now()
		if !pace.due(now) {
			return
		}
		hours, err := p.cfg.Elvia.Hours(ctx, startOfMonth(now), now.Truncate(time.Hour).Add(time.Hour))
		pace.done(now, err)
		if err != nil {
			slog.Error("could not fetch consumption from Elvia", "err", err, "retry_at", pace.next.Format(time.TimeOnly))
			return
		}
		usage := make([]Usage, len(hours))
		for i, h := range hours {
			usage[i] = Usage{Start: h.Start, Consumption: h.KWh}
		}
		p.storeHours(Home, usage, now)
		slog.Debug("consumption fetched", "place", Home, "hours", len(usage))
	})
}

// runTibberHours fetches the cabin's hours for the month so far shortly
// after start and then once every hour of the clock. The history isn't
// real-time, so polling more often is pointless.
func (p *Power) runTibberHours(ctx context.Context) {
	if !schedule.Sleep(ctx, 2*time.Second) { // let the feed deliver today's total first
		return
	}
	var pace monthlyPace
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		now := tz.Now()
		if !pace.due(now) {
			return
		}
		hours, err := p.cfg.Tibber.Hours(ctx, now.Day()*24+now.Hour()+3)
		if err != nil {
			slog.Error("could not fetch consumption from Tibber", "place", Cabin, "err", err)
		} else {
			usage := make([]Usage, len(hours))
			for i, h := range hours {
				usage[i] = Usage{Start: h.Start, Consumption: h.KWh}
			}
			p.storeHours(Cabin, usage, now)
			slog.Debug("consumption fetched", "place", Cabin, "hours", len(usage))
		}
		pace.done(now, err)
	})
}

// monthlyPace says when to fetch next: once an hour, and when that fails, soon
// again with the backoff Tibber requires, since a failure at midnight would
// otherwise leave the numbers wrong for an hour.
type monthlyPace struct {
	hour     time.Time // the hour of the last success
	failures int
	next     time.Time // no retry before this
}

func (m *monthlyPace) due(now time.Time) bool {
	return !now.Truncate(time.Hour).Equal(m.hour) && !now.Before(m.next)
}

func (m *monthlyPace) done(now time.Time, err error) {
	if err != nil && !errors.Is(err, tibber.ErrUnauthorized) {
		m.failures++
		m.next = now.Add(schedule.Backoff(m.failures))
		return
	}
	// Tibber says not to retry a rejected token, so that waits for its hour too.
	m.hour, m.failures, m.next = now.Truncate(time.Hour), 0, time.Time{}
}
