// Package calendar keeps events, birthdays, dinners and the kids' schedule
// from Google Calendar in memory.
package calendar

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

type Source string

const (
	Felles Source = "felles"
	Audun  Source = "audun"
)

type Event struct {
	Title   string
	Start   time.Time
	End     time.Time // for full-day events: midnight of the last day
	FullDay bool
	Source  Source // only set for the two main event calendars
}

// IDs of the Google calendars to read. Empty IDs are skipped.
type Config struct {
	Felles, Audun, Barneuker, Bursdag, Middag, Kindergarden string
}

// Snapshot is an immutable view of all calendar data at one point in time.
// Refreshes swap in new slices rather than mutating old ones, so a Snapshot
// can be read without holding any lock.
type Snapshot struct {
	Events    []Event // felles + audun, sorted by start
	Birthdays []Event
	Dinners   []Event
	Barneuker []Event
	// Number of "bhg" days left until kindergarten is done (nil until fetched)
	BhgDaysLeft *int
}

// fetchFunc lists visible events in a calendar from now until timeMax.
type fetchFunc func(ctx context.Context, calendarID string, timeMax time.Time) ([]Event, error)

type Calendar struct {
	cfg   Config
	fetch fetchFunc

	mu   sync.RWMutex
	data Snapshot
}

// New creates a calendar that reads with the given service-account key JSON.
func New(ctx context.Context, cfg Config, serviceAccountJSON []byte) (*Calendar, error) {
	svc, err := gcal.NewService(ctx,
		option.WithAuthCredentialsJSON(option.ServiceAccount, serviceAccountJSON),
		option.WithScopes(gcal.CalendarReadonlyScope),
	)
	if err != nil {
		return nil, fmt.Errorf("google calendar: %w", err)
	}
	return &Calendar{cfg: cfg, fetch: googleFetcher(svc)}, nil
}

// Snapshot returns the current calendar data.
func (c *Calendar) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// Run refreshes events every minute, and birthdays, dinners, barneuker and
// the kindergarten count every 15 minutes, until ctx is cancelled.
func (c *Calendar) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { schedule.Every(ctx, time.Minute, c.refreshEvents) })
	wg.Go(func() {
		schedule.Every(ctx, 15*time.Minute, func(ctx context.Context) {
			c.refreshList(ctx, "birthdays", c.cfg.Bursdag, func(s *Snapshot, e []Event) { s.Birthdays = e })
			c.refreshList(ctx, "dinners", c.cfg.Middag, func(s *Snapshot, e []Event) { s.Dinners = e })
			c.refreshList(ctx, "barneuker", c.cfg.Barneuker, func(s *Snapshot, e []Event) { s.Barneuker = e })
			c.refreshBhgCount(ctx)
		})
	})
	wg.Wait()
}

func (c *Calendar) update(fn func(*Snapshot)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(&c.data)
}

func twoWeeksAhead() time.Time { return tz.Now().AddDate(0, 0, 14) }

func (c *Calendar) refreshEvents(ctx context.Context) {
	sources := []struct {
		id     string
		source Source
	}{{c.cfg.Felles, Felles}, {c.cfg.Audun, Audun}}

	var all []Event
	for _, s := range sources {
		if s.id == "" {
			continue
		}
		events, err := c.fetch(ctx, s.id, twoWeeksAhead())
		if err != nil {
			slog.Error("could not fetch events; keeping previous", "calendar", s.source, "err", err)
			return
		}
		for i := range events {
			events[i].Source = s.source
		}
		all = append(all, events...)
	}
	sortByStart(all)
	c.update(func(s *Snapshot) { s.Events = all })
	slog.Info("events fetched", "count", len(all))
}

func (c *Calendar) refreshList(ctx context.Context, name, id string, set func(*Snapshot, []Event)) {
	if id == "" {
		c.update(func(s *Snapshot) { set(s, nil) })
		return
	}
	events, err := c.fetch(ctx, id, twoWeeksAhead())
	if err != nil {
		slog.Error("could not fetch calendar; keeping previous", "calendar", name, "err", err)
		return
	}
	c.update(func(s *Snapshot) { set(s, events) })
	slog.Info("calendar fetched", "calendar", name, "count", len(events))
}

// refreshBhgCount counts remaining kindergarten ("bhg") days until Engebret
// is done, fetching the childcare calendar through August 2027.
func (c *Calendar) refreshBhgCount(ctx context.Context) {
	if c.cfg.Kindergarden == "" {
		c.update(func(s *Snapshot) { s.BhgDaysLeft = nil })
		return
	}
	events, err := c.fetch(ctx, c.cfg.Kindergarden, time.Date(2027, 9, 1, 0, 0, 0, 0, tz.Oslo))
	if err != nil {
		slog.Error("could not fetch kindergarten calendar", "err", err)
		return
	}
	n := 0
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Title), "bhg") {
			n++
		}
	}
	c.update(func(s *Snapshot) { s.BhgDaysLeft = &n })
	slog.Info("bhg days left", "count", n)
}

func googleFetcher(svc *gcal.Service) fetchFunc {
	return func(ctx context.Context, calendarID string, timeMax time.Time) ([]Event, error) {
		var out []Event
		call := svc.Events.List(calendarID).
			TimeMin(tz.Now().Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			MaxResults(2000).
			SingleEvents(true).
			OrderBy("startTime")
		err := call.Pages(ctx, func(page *gcal.Events) error {
			for _, item := range page.Items {
				if !isVisible(item) {
					continue
				}
				e, err := parseGoogleEvent(item)
				if err != nil {
					slog.Warn("skipping event", "calendar", calendarID, "err", err)
					continue
				}
				out = append(out, e)
			}
			return nil
		})
		return out, err
	}
}

// isVisible: events this calendar organized always show. Invitations from
// others only show once accepted or tentatively accepted, so declined and
// unanswered ones are hidden. Google marks the calendar's own entries with
// Self.
func isVisible(e *gcal.Event) bool {
	if e.Organizer != nil && e.Organizer.Self {
		return true
	}
	for _, a := range e.Attendees {
		if a.Self {
			return a.ResponseStatus == "accepted" || a.ResponseStatus == "tentative"
		}
	}
	return true
}

func parseGoogleEvent(e *gcal.Event) (Event, error) {
	if e.Start == nil || e.End == nil {
		return Event{}, fmt.Errorf("event %q has no start or end", e.Summary)
	}
	if e.Start.Date != "" && e.End.Date != "" {
		// Full day. Google's end date is exclusive, so step back one day.
		start, err1 := time.ParseInLocation(time.DateOnly, e.Start.Date, tz.Oslo)
		end, err2 := time.ParseInLocation(time.DateOnly, e.End.Date, tz.Oslo)
		if err1 != nil || err2 != nil {
			return Event{}, fmt.Errorf("event %q has invalid dates", e.Summary)
		}
		return Event{Title: e.Summary, Start: start, End: end.AddDate(0, 0, -1), FullDay: true}, nil
	}
	start, err1 := time.Parse(time.RFC3339, e.Start.DateTime)
	end, err2 := time.Parse(time.RFC3339, e.End.DateTime)
	if err1 != nil || err2 != nil {
		return Event{}, fmt.Errorf("event %q has invalid times", e.Summary)
	}
	return Event{Title: e.Summary, Start: start.In(tz.Oslo), End: end.In(tz.Oslo)}, nil
}
