package calendar

import (
	"context"
	"testing"
	"time"

	gcal "google.golang.org/api/calendar/v3"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func TestIsVisible(t *testing.T) {
	other := "other@example.com"
	invitation := func(status string, organizerSelf bool) *gcal.Event {
		return &gcal.Event{
			Summary:   "Møte",
			Organizer: &gcal.EventOrganizer{Email: other, Self: organizerSelf},
			Attendees: []*gcal.EventAttendee{
				{Email: other, ResponseStatus: "accepted"},
				{Self: true, ResponseStatus: status},
			},
		}
	}
	cases := []struct {
		name  string
		event *gcal.Event
		want  bool
	}{
		{"own event without guests", &gcal.Event{Summary: "Egen"}, true},
		{"own event with empty guest list", &gcal.Event{Summary: "Egen", Attendees: []*gcal.EventAttendee{}}, true},
		{"organized by me, declined", invitation("declined", true), true},
		{"organized by me, unanswered", invitation("needsAction", true), true},
		{"accepted invitation", invitation("accepted", false), true},
		{"tentative invitation", invitation("tentative", false), true},
		{"declined invitation", invitation("declined", false), false},
		{"unanswered invitation", invitation("needsAction", false), false},
	}
	for _, c := range cases {
		if got := isVisible(c.event); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	e := invitation("accepted", false)
	e.Attendees[0].ResponseStatus = "declined"
	if !isVisible(e) {
		t.Error("another guest's status should not matter")
	}
}

// The kids are exchanged at 16:00 on days where we switch.
var day = time.Date(2026, 3, 10, 0, 0, 0, 0, tz.Oslo)

func at(h, m int) time.Time { return time.Date(2026, 3, 10, h, m, 0, 0, tz.Oslo) }

func felles(start, end time.Time) Event {
	return Event{Title: "Noe felles", Start: start, End: end, Source: Felles}
}

func TestIsFaded(t *testing.T) {
	fullDay := felles(day, day)
	fullDay.FullDay = true
	own := felles(at(18, 0), at(19, 0))
	own.Source = Audun
	middleDay := felles(time.Date(2026, 3, 9, 20, 0, 0, 0, tz.Oslo), time.Date(2026, 3, 11, 8, 0, 0, 0, tz.Oslo))
	lastDay := felles(time.Date(2026, 3, 9, 20, 0, 0, 0, tz.Oslo), at(9, 0))
	firstDay := felles(at(17, 0), time.Date(2026, 3, 11, 8, 0, 0, 0, tz.Oslo))

	cases := []struct {
		name  string
		event Event
		kids  KidsStatus
		want  bool
	}{
		// Kids at Hanne all week: always faded.
		{"away, morning", felles(at(9, 0), at(10, 0)), KidsAway, true},
		{"away, evening", felles(at(18, 0), at(19, 0)), KidsAway, true},
		// Kids at Audun all week: never faded.
		{"full, morning", felles(at(9, 0), at(10, 0)), KidsFull, false},
		{"full, evening", felles(at(18, 0), at(19, 0)), KidsFull, false},
		// Leaving (Audun to Hanne): faded only after the handover.
		{"leaving 09:00", felles(at(9, 0), at(10, 0)), KidsLeaving, false},
		{"leaving 15:59", felles(at(15, 59), at(17, 0)), KidsLeaving, false},
		{"leaving 16:00", felles(at(16, 0), at(17, 0)), KidsLeaving, true},
		{"leaving 18:00", felles(at(18, 0), at(19, 0)), KidsLeaving, true},
		// Arriving (Hanne to Audun): faded only before the handover.
		{"arriving 09:00", felles(at(9, 0), at(10, 0)), KidsArriving, true},
		{"arriving 15:59", felles(at(15, 59), at(17, 0)), KidsArriving, true},
		{"arriving 16:00", felles(at(16, 0), at(17, 0)), KidsArriving, false},
		{"arriving 18:00", felles(at(18, 0), at(19, 0)), KidsArriving, false},
		// Audun's own events are never faded.
		{"own, away", own, KidsAway, false},
		{"own, leaving", own, KidsLeaving, false},
		{"own, arriving", own, KidsArriving, false},
		// Full-day events span the day, so exchange days render them black.
		{"full day, leaving", fullDay, KidsLeaving, false},
		{"full day, arriving", fullDay, KidsArriving, false},
		{"full day, full", fullDay, KidsFull, false},
		{"full day, away", fullDay, KidsAway, true},
		// A multi-day event that started earlier spans the day.
		{"middle day, leaving", middleDay, KidsLeaving, false},
		{"middle day, arriving", middleDay, KidsArriving, false},
		{"last day, leaving", lastDay, KidsLeaving, false},
		{"last day, arriving", lastDay, KidsArriving, false},
		// An event starting today and running into tomorrow uses its start time.
		{"first day, leaving", firstDay, KidsLeaving, true},
		{"first day, arriving", firstDay, KidsArriving, false},
	}
	for _, c := range cases {
		if got := isFaded(c.event, day, c.kids); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestKidsStatus(t *testing.T) {
	barneuke := func(title string) Event { return Event{Title: title, Start: day, End: day, FullDay: true} }
	cases := map[string]KidsStatus{
		"Audun → Hanne": KidsLeaving,
		"Hanne → Audun": KidsArriving,
		"Audun":         KidsFull,
		"Hanne":         KidsAway,
	}
	for title, want := range cases {
		s := Snapshot{Barneuker: []Event{barneuke(title)}}
		if got := s.KidsStatusForDate(day); got != want {
			t.Errorf("%q: got %q, want %q", title, got, want)
		}
	}
}

func TestDisplayTitle(t *testing.T) {
	long := "En veldig lang tittel som går langt over femti tegn, æøå"
	cases := []struct {
		title string
		k     kind
		want  string
	}{
		{"Ola Nordmann 1980", kindBirthday, "Ola Nordmann (46 år)"},
		{"Bestemor", kindBirthday, "Bestemor"},
		{long, kindEvent, string([]rune(long)[:49]) + "…"},
		{long, kindDinner, long},
		{"", kindEvent, "(uten tittel)"},
	}
	for _, c := range cases {
		if got := displayTitle(Event{Title: c.title}, c.k, 2026); got != c.want {
			t.Errorf("%q: got %q, want %q", c.title, got, c.want)
		}
	}
}

func TestDisplayTime(t *testing.T) {
	multi := felles(time.Date(2026, 3, 9, 20, 0, 0, 0, tz.Oslo), time.Date(2026, 3, 11, 8, 30, 0, 0, tz.Oslo))
	cases := []struct {
		date time.Time
		want DisplayTime
	}{
		{time.Date(2026, 3, 9, 12, 0, 0, 0, tz.Oslo), DisplayTime{"20:00", "", "–"}},
		{day, DisplayTime{"", "", "..."}},
		{time.Date(2026, 3, 11, 12, 0, 0, 0, tz.Oslo), DisplayTime{"", "08:30", "–"}},
	}
	for _, c := range cases {
		if got := displayTime(multi, getDayType(multi, c.date)); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.date.Format(time.DateOnly), got, c.want)
		}
	}
	single := felles(at(9, 5), at(10, 0))
	if got := displayTime(single, getDayType(single, day)); got != (DisplayTime{"09:05", "10:00", "–"}) {
		t.Errorf("single day: got %+v", got)
	}
}

func TestParseFullDayEvent(t *testing.T) {
	e, err := parseGoogleEvent(&gcal.Event{
		Summary: "Hytta",
		Start:   &gcal.EventDateTime{Date: "2026-03-10"},
		End:     &gcal.EventDateTime{Date: "2026-03-12"}, // exclusive
	})
	if err != nil {
		t.Fatal(err)
	}
	if !e.FullDay || !e.Start.Equal(day) || !e.End.Equal(day.AddDate(0, 0, 1)) {
		t.Errorf("got %+v", e)
	}
}

func TestRefreshEventsTagsSourcesAndKeepsOldDataOnError(t *testing.T) {
	failing := false
	c := &Calendar{
		cfg: Config{Felles: "f", Audun: "a"},
		fetch: func(_ context.Context, id string, _ time.Time) ([]Event, error) {
			if failing {
				return nil, context.DeadlineExceeded
			}
			if id == "f" {
				return []Event{{Title: "late", Start: at(18, 0), End: at(19, 0)}}, nil
			}
			return []Event{{Title: "early", Start: at(8, 0), End: at(9, 0)}}, nil
		},
	}
	c.refreshEvents(context.Background())
	got := c.Snapshot().Events
	if len(got) != 2 || got[0].Title != "early" || got[0].Source != Audun || got[1].Source != Felles {
		t.Fatalf("got %+v", got)
	}
	failing = true
	c.refreshEvents(context.Background())
	if len(c.Snapshot().Events) != 2 {
		t.Error("a failed refresh should keep the previous events")
	}
}
