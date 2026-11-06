package spond

import (
	"slices"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// nagDays is how close an event or its reply deadline has to be before a
// missing reply is shown. Most sessions never get a reply, so showing it
// further ahead would mark almost everything.
const nagDays = 7

// merge adds Spond's events to a snapshot. An event Felles already has stays
// one line, with Felles' title; the others get a line of their own. They all
// count as Felles, which is what dims them when the kids are away.
func merge(snap calendar.Snapshot, events []Event, now time.Time) calendar.Snapshot {
	if len(events) == 0 {
		return snap
	}
	out := slices.Clone(snap.Events)
	for _, s := range events {
		i := slices.IndexFunc(snap.Events, func(e calendar.Event) bool { return isSame(e, s) })
		if i >= 0 {
			out[i].Mark, out[i].Detail = describe(snap, snap.Events[i], s, i, now)
			continue
		}
		line := calendar.Event{
			Title:       s.Child + ": " + s.Heading,
			Location:    s.Location,
			Description: s.Description,
			Start:       s.Start,
			End:         s.End,
			Source:      calendar.Felles,
		}
		line.Mark, line.Detail = describe(snap, line, s, -1, now)
		out = append(out, line)
	}
	slices.SortStableFunc(out, func(a, b calendar.Event) int { return a.Start.Compare(b.Start) })
	snap.Events = out
	return snap
}

// isSame recognises Felles' own entry for an event. The titles have nothing
// in common ("Agnes på turn", "TURN 5.-7. klasse"), so it goes by the time and
// the child's name.
func isSame(e calendar.Event, s Event) bool {
	if e.Source != calendar.Felles || e.FullDay || !mentions(e, s.Child) {
		return false
	}
	return e.Start.Equal(s.Start) || e.Start.Equal(s.Meetup)
}

func mentions(e calendar.Event, child string) bool {
	return strings.Contains(strings.ToLower(e.Title), strings.ToLower(child))
}

// describe says how s stands: briefly for the screen, in words for Claude.
// own is the index in snap.Events of s's own line, -1 if it has none there.
func describe(snap calendar.Snapshot, line calendar.Event, s Event, own int, now time.Time) (mark, detail string) {
	clash := ""
	if s.Reply == Unanswered && !deadlinePassed(s, now) {
		clash = clashWith(snap, s, own)
	}
	var words []string
	switch s.Reply {
	case Accepted:
		mark = "✓"
		words = append(words, "svart ja i Spond")
	case Declined:
		mark = "✕"
		words = append(words, "svart nei i Spond")
	case Waiting:
		mark = "venteliste"
		words = append(words, "på venteliste i Spond")
	default:
		if clash != "" {
			mark = "kolliderer"
		} else if needsReply(s, now) && !snap.FadedOn(line, s.Start) {
			mark = "ikke svart"
		}
		words = append(words, "ikke svart i Spond")
	}
	if !s.Meetup.IsZero() {
		words = append(words, "oppmøte "+s.Meetup.In(tz.Oslo).Format("15:04"))
	}
	if clash != "" {
		words = append(words, "kolliderer med "+clash)
	}
	return mark, strings.Join(words, ", ")
}

func deadlinePassed(s Event, now time.Time) bool {
	return !s.RSVPBy.IsZero() && now.After(s.RSVPBy)
}

func needsReply(s Event, now time.Time) bool {
	if deadlinePassed(s, now) {
		return false
	}
	limit := now.AddDate(0, 0, nagDays)
	return !s.Start.After(limit) || (!s.RSVPBy.IsZero() && !s.RSVPBy.After(limit))
}

// clashWith names something else in Felles about the same child at the same
// time, "" if there is nothing.
func clashWith(snap calendar.Snapshot, s Event, own int) string {
	for i, e := range snap.Events {
		if i == own || e.Source != calendar.Felles || !mentions(e, s.Child) {
			continue
		}
		end := e.End
		if e.FullDay {
			end = tz.StartOfDay(e.End).AddDate(0, 0, 1) // End is midnight of the last day
		}
		if e.Start.Before(s.End) && s.Start.Before(end) {
			return e.Title
		}
	}
	return ""
}
