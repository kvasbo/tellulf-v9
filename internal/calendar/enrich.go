package calendar

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// KidsStatus says where the kids are on a given day.
type KidsStatus string

const (
	KidsAway     KidsStatus = ""         // no matching barneuker event
	KidsFull     KidsStatus = "full"     // with Audun all day
	KidsLeaving  KidsStatus = "leaving"  // Audun to Hanne at handover
	KidsArriving KidsStatus = "arriving" // Hanne to Audun at handover
)

// DayType says how an event relates to the day it is shown on.
type DayType string

const (
	SingleDay DayType = "singleDay"
	FirstDay  DayType = "firstDay"
	LastDay   DayType = "lastDay"
	MiddleDay DayType = "middleDay"
)

type DisplayTime struct{ Start, End, Spacer string }

type EnrichedEvent struct {
	Event
	DisplayTitle string
	DayType      DayType
	DisplayTime  DisplayTime
	Faded        bool
}

type kind int

const (
	kindEvent kind = iota
	kindBirthday
	kindDinner
)

// The kids are exchanged at 16:00 on the days we switch.
const handoverHour = 16

func sortByStart(events []Event) {
	slices.SortStableFunc(events, func(a, b Event) int { return a.Start.Compare(b.Start) })
}

func (s Snapshot) EventsForDate(d time.Time) []EnrichedEvent {
	return s.forDate(s.Events, kindEvent, d)
}

func (s Snapshot) BirthdaysForDate(d time.Time) []EnrichedEvent {
	return s.forDate(s.Birthdays, kindBirthday, d)
}

func (s Snapshot) DinnersForDate(d time.Time) []EnrichedEvent {
	return s.forDate(s.Dinners, kindDinner, d)
}

func (s Snapshot) forDate(events []Event, k kind, d time.Time) []EnrichedEvent {
	kids := s.KidsStatusForDate(d)
	var out []EnrichedEvent
	for _, e := range events {
		if !occursOn(e, d) {
			continue
		}
		dayType := getDayType(e, d)
		out = append(out, EnrichedEvent{
			Event:        e,
			DisplayTitle: displayTitle(e, k, tz.Now().Year()),
			DayType:      dayType,
			DisplayTime:  displayTime(e, dayType),
			Faded:        isFaded(e, d, kids),
		})
	}
	return out
}

// KidsStatusForDate looks at full-day barneuker events on d:
//   - leaving  when the title has both names and "audun" comes first
//   - arriving when the title has both names and "hanne" comes first
//   - full     when the title only mentions "audun"
func (s Snapshot) KidsStatusForDate(d time.Time) KidsStatus {
	var titles []string
	for _, e := range s.Barneuker {
		if e.FullDay && occursOn(e, d) {
			titles = append(titles, strings.ToLower(e.Title))
		}
	}
	for _, t := range titles {
		a, h := strings.Index(t, "audun"), strings.Index(t, "hanne")
		if a != -1 && h != -1 {
			if a < h {
				return KidsLeaving
			}
			return KidsArriving
		}
	}
	for _, t := range titles {
		if strings.Contains(t, "audun") {
			return KidsFull
		}
	}
	return KidsAway
}

// occursOn reports whether the event touches the calendar date of d.
func occursOn(e Event, d time.Time) bool {
	day := tz.StartOfDay(d)
	return !tz.StartOfDay(e.Start).After(day) && !tz.StartOfDay(e.End).Before(day)
}

// getDayType figures out whether an event is within one day or spans several,
// and if so which part of it d is.
func getDayType(e Event, d time.Time) DayType {
	switch {
	case tz.SameDay(e.Start, e.End):
		return SingleDay
	case tz.SameDay(e.Start, d):
		return FirstDay
	case tz.SameDay(e.End, d):
		return LastDay
	default:
		return MiddleDay
	}
}

func displayTime(e Event, dayType DayType) DisplayTime {
	const arrow = "–"
	hhmm := func(t time.Time) string { return t.In(tz.Oslo).Format("15:04") }
	switch dayType {
	case MiddleDay:
		return DisplayTime{Spacer: "..."}
	case LastDay:
		return DisplayTime{End: hhmm(e.End), Spacer: arrow}
	case FirstDay:
		return DisplayTime{Start: hhmm(e.Start), Spacer: arrow}
	default:
		return DisplayTime{Start: hhmm(e.Start), End: hhmm(e.End), Spacer: arrow}
	}
}

var birthYearPattern = regexp.MustCompile(`(?i)[A-Za-z0-9 ]+\s[0-9]+`)

// displayTitle turns "Ola Nordmann 1980" into "Ola Nordmann (46 år)" for
// birthdays, and shortens long event titles.
func displayTitle(e Event, k kind, thisYear int) string {
	title := []rune(e.Title)
	switch {
	case k == kindBirthday && birthYearPattern.MatchString(e.Title) && len(title) >= 5:
		if year, err := strconv.Atoi(string(title[len(title)-4:])); err == nil {
			return string(title[:len(title)-5]) + " (" + strconv.Itoa(thisYear-year) + " år)"
		}
	case k == kindEvent && len(title) > 50:
		return string(title[:49]) + "…"
	}
	if len(title) == 0 {
		return "(uten tittel)"
	}
	return e.Title
}

// isFaded says whether a felles event should be greyed out, i.e. whether the
// kids are away while it happens. On exchange days the handover at 16:00
// splits the day. Events that span the day rather than start in it have no
// start time to compare, so they stay black whenever the kids are here at all.
func isFaded(e Event, d time.Time, kids KidsStatus) bool {
	if e.Source != Felles {
		return false
	}
	switch kids {
	case KidsAway:
		return true
	case KidsFull:
		return false
	}
	dayType := getDayType(e, d)
	if e.FullDay || dayType == MiddleDay || dayType == LastDay {
		return false
	}
	afterHandover := e.Start.In(tz.Oslo).Hour() >= handoverHour
	if kids == KidsLeaving {
		return afterHandover
	}
	return !afterHandover
}
