package spond

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

var (
	// A Monday, with the kids here all week unless a test says otherwise.
	mergeDay = time.Date(2026, 11, 9, 0, 0, 0, 0, tz.Oslo)
	mergeNow = mergeDay.Add(8 * time.Hour)
	turnAt   = mergeDay.Add(17 * time.Hour)
)

func kidsHere(from time.Time, days int) []calendar.Event {
	return []calendar.Event{{Title: "Audun", Start: from, End: from.AddDate(0, 0, days), FullDay: true}}
}

func felles(title string, start time.Time, d time.Duration) calendar.Event {
	return calendar.Event{Title: title, Start: start, End: start.Add(d), Source: calendar.Felles}
}

func fellesFullDay(title string, from, to time.Time) calendar.Event {
	return calendar.Event{Title: title, Start: from, End: to, FullDay: true, Source: calendar.Felles}
}

func turn(start time.Time, r Reply) Event {
	return Event{Child: "Agnes", Heading: "TURN 5.-7. klasse", Start: start, End: start.Add(90 * time.Minute), Reply: r}
}

func snapshot(events ...calendar.Event) calendar.Snapshot {
	return calendar.Snapshot{Events: events, Barneuker: kidsHere(mergeDay.AddDate(0, 0, -1), 30)}
}

// "Agnes på turn" in Felles and "TURN 5.-7. klasse" in Spond are the same
// thing: they stay one line, with Felles' title and Spond's reply.
func TestDuplicateStaysOneLineWithTheReply(t *testing.T) {
	got := merge(snapshot(felles("Agnes på turn", turnAt, 90*time.Minute)), []Event{turn(turnAt, Accepted)}, mergeNow).Events
	if len(got) != 1 {
		t.Fatalf("%d lines: %+v", len(got), got)
	}
	if got[0].Title != "Agnes på turn" || got[0].Mark != "✓" {
		t.Errorf("got %q with mark %q", got[0].Title, got[0].Mark)
	}
}

func TestWhatCountsAsADuplicate(t *testing.T) {
	audun := felles("Agnes på turn", turnAt, time.Hour)
	audun.Source = calendar.Audun
	meetup := turn(turnAt, Accepted)
	meetup.Meetup = turnAt.Add(-15 * time.Minute)

	cases := []struct {
		name  string
		event calendar.Event
		spond Event
		lines int
	}{
		{"same start, child named", felles("Agnes på turn", turnAt, time.Hour), turn(turnAt, Accepted), 1},
		{"name in another case", felles("AGNES turn", turnAt, time.Hour), turn(turnAt, Accepted), 1},
		{"Felles at the meeting time", felles("Agnes turn", turnAt.Add(-15*time.Minute), time.Hour), meetup, 1},
		{"another child at the same time", felles("Jonas fotball", turnAt, time.Hour), turn(turnAt, Accepted), 2},
		{"same child at another time", felles("Agnes tannlege", turnAt.Add(30*time.Minute), time.Hour), turn(turnAt, Accepted), 2},
		{"Audun's own calendar", audun, turn(turnAt, Accepted), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := len(merge(snapshot(c.event), []Event{c.spond}, mergeNow).Events); got != c.lines {
				t.Errorf("%d lines, want %d", got, c.lines)
			}
		})
	}
}

// What Felles doesn't have gets its own line, in time order, saying whose
// it is, and is dimmed like Felles when the kids are away.
func TestOnlyInSpondGetsItsOwnLine(t *testing.T) {
	e := turn(turnAt, Accepted)
	e.Location, e.Description = "Njårdhallen", "Ta med drikke"
	got := merge(snapshot(felles("Middag ute", turnAt.Add(3*time.Hour), time.Hour), felles("Tannlege", mergeDay.Add(9*time.Hour), time.Hour)),
		[]Event{e}, mergeNow).Events

	var titles []string
	for _, ev := range got {
		titles = append(titles, ev.Title)
	}
	if want := []string{"Tannlege", "Agnes: TURN 5.-7. klasse", "Middag ute"}; !slices.Equal(titles, want) {
		t.Fatalf("got %q, want %q", titles, want)
	}
	line := got[1]
	if line.Title != "Agnes: TURN 5.-7. klasse" || line.Location != "Njårdhallen" || line.Description != "Ta med drikke" {
		t.Errorf("line: %+v", line)
	}
	if !line.Start.Equal(turnAt) || !line.End.Equal(turnAt.Add(90*time.Minute)) {
		t.Errorf("times: %v – %v", line.Start, line.End)
	}
	if line.Source != calendar.Felles {
		t.Errorf("source %q: only Felles events are dimmed when the kids are away", line.Source)
	}
}

func TestMarks(t *testing.T) {
	soon := turnAt.AddDate(0, 0, 3)
	inSevenDays := mergeNow.AddDate(0, 0, 7)
	later := turnAt.AddDate(0, 0, 10)
	withDeadline := func(start, by time.Time) Event {
		e := turn(start, Unanswered)
		e.RSVPBy = by
		return e
	}
	cases := []struct {
		name  string
		event Event
		want  string
	}{
		{"accepted", turn(soon, Accepted), "✓"},
		{"declined", turn(soon, Declined), "✕"},
		{"on the waiting list", turn(soon, Waiting), "venteliste"},
		{"no reply this week", turn(soon, Unanswered), "ikke svart"},
		{"no reply at exactly seven days", turn(inSevenDays, Unanswered), "ikke svart"},
		{"no reply, just past seven days", turn(inSevenDays.Add(time.Minute), Unanswered), ""},
		{"no reply far ahead", turn(later, Unanswered), ""},
		{"far ahead, accepted", turn(later, Accepted), "✓"},
		{"far ahead, deadline soon", withDeadline(later, mergeNow.AddDate(0, 0, 2)), "ikke svart"},
		{"far ahead, deadline far off", withDeadline(later, mergeNow.AddDate(0, 0, 8)), ""},
		{"deadline has passed", withDeadline(soon, mergeNow.Add(-time.Hour)), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := merge(calendar.Snapshot{Barneuker: kidsHere(mergeDay, 30)}, []Event{c.event}, mergeNow).Events
			if got[0].Mark != c.want {
				t.Errorf("mark %q, want %q", got[0].Mark, c.want)
			}
		})
	}
}

// Dimmed events are for the other home: no nagging, but what is answered
// still shows.
func TestNoNagWhenTheKidsAreAway(t *testing.T) {
	away := calendar.Snapshot{} // no barneuker on the day: the kids are away
	if got := merge(away, []Event{turn(turnAt, Unanswered)}, mergeNow).Events[0].Mark; got != "" {
		t.Errorf("unanswered: mark %q", got)
	}
	if got := merge(away, []Event{turn(turnAt, Accepted)}, mergeNow).Events[0].Mark; got != "✓" {
		t.Errorf("accepted: mark %q", got)
	}
}

func TestConflicts(t *testing.T) {
	berlinFrom, berlinTo := mergeDay.AddDate(0, 0, 10), mergeDay.AddDate(0, 0, 13) // Thu to Sun
	duringBerlin := berlinFrom.AddDate(0, 0, 1).Add(17 * time.Hour)
	cases := []struct {
		name  string
		other calendar.Event
		event Event
		want  string
	}{
		{"trip with the child's name", fellesFullDay("Agnes og Audun i Berlin", berlinFrom, berlinTo), turn(duringBerlin, Unanswered), "kolliderer"},
		{"the trip's last day counts", fellesFullDay("Agnes og Audun i Berlin", berlinFrom, berlinFrom), turn(berlinFrom.Add(17*time.Hour), Unanswered), "kolliderer"},
		{"the day after the trip", fellesFullDay("Agnes og Audun i Berlin", berlinFrom, berlinFrom), turn(berlinFrom.AddDate(0, 0, 1), Unanswered), ""},
		{"overlapping appointment", felles("Legetime Agnes", turnAt.Add(30*time.Minute), time.Hour), turn(turnAt, Unanswered), "kolliderer"},
		{"appointment ends as it starts", felles("Legetime Agnes", turnAt.Add(-time.Hour), time.Hour), turn(turnAt, Unanswered), "ikke svart"},
		{"someone else's trip", fellesFullDay("Jonas på tur", berlinFrom, berlinTo), turn(duringBerlin, Unanswered), ""},
		{"already answered", fellesFullDay("Agnes og Audun i Berlin", berlinFrom, berlinTo), turn(duringBerlin, Declined), "✕"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := findSpondLine(t, merge(snapshot(c.other), []Event{c.event}, mergeNow).Events)
			if line.Mark != c.want {
				t.Errorf("mark %q, want %q", line.Mark, c.want)
			}
		})
	}

	// A clash still shows when the kids are away: the trip is what decides.
	line := findSpondLine(t, merge(calendar.Snapshot{Events: []calendar.Event{fellesFullDay("Agnes og Audun i Berlin", berlinFrom, berlinTo)}},
		[]Event{turn(duringBerlin, Unanswered)}, mergeNow).Events)
	if line.Mark != "kolliderer" {
		t.Errorf("kids away: mark %q", line.Mark)
	}

	// The Felles copy of the same session is no clash with itself.
	dup := merge(snapshot(felles("Agnes på turn", turnAt, 90*time.Minute)), []Event{turn(turnAt, Unanswered)}, mergeNow).Events[0]
	if dup.Mark != "ikke svart" {
		t.Errorf("duplicate: mark %q", dup.Mark)
	}
}

func findSpondLine(t *testing.T, events []calendar.Event) calendar.Event {
	t.Helper()
	for _, e := range events {
		if e.Title == "Agnes: TURN 5.-7. klasse" {
			return e
		}
	}
	t.Fatalf("no line from Spond in %+v", events)
	return calendar.Event{}
}

// The words Claude's sentence is written from.
func TestDetail(t *testing.T) {
	withMeetup := turn(turnAt, Accepted)
	withMeetup.Meetup = turnAt.Add(-30 * time.Minute)
	clash := turn(turnAt, Unanswered)
	cases := []struct {
		name  string
		other []calendar.Event
		event Event
		want  string
	}{
		{"no reply", nil, turn(turnAt, Unanswered), "ikke svart i Spond"},
		{"yes", nil, turn(turnAt, Accepted), "svart ja i Spond"},
		{"no", nil, turn(turnAt, Declined), "svart nei i Spond"},
		{"waiting", nil, turn(turnAt, Waiting), "på venteliste i Spond"},
		{"meeting time", nil, withMeetup, "svart ja i Spond, oppmøte 16:30"},
		{"clash", []calendar.Event{felles("Legetime Agnes", turnAt.Add(30*time.Minute), time.Hour)}, clash, "ikke svart i Spond, kolliderer med Legetime Agnes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := findSpondLine(t, merge(snapshot(c.other...), []Event{c.event}, mergeNow).Events)
			if line.Detail != c.want {
				t.Errorf("got %q, want %q", line.Detail, c.want)
			}
		})
	}
}

// Snapshots are shared between readers, so the merge works on a copy.
func TestMergeLeavesTheSnapshotAlone(t *testing.T) {
	snap := snapshot(felles("Agnes på turn", turnAt, 90*time.Minute))
	merge(snap, []Event{turn(turnAt, Accepted), {Child: "Jonas", Heading: "Trening", Start: turnAt, End: turnAt}}, mergeNow)
	if len(snap.Events) != 1 || snap.Events[0].Mark != "" {
		t.Errorf("changed: %+v", snap.Events)
	}
}

// Without Spond's data the screen is as it would be without Spond.
func TestNoEventsChangeNothing(t *testing.T) {
	snap := snapshot(felles("Agnes på turn", turnAt, 90*time.Minute))
	got := merge(snap, nil, mergeNow)
	if !slices.Equal(got.Events, snap.Events) {
		t.Errorf("got %+v", got.Events)
	}
}
