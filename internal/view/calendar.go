package view

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Calendar is the right column: today in full, the coming days on two lines
// each, and the kindergarten counter for the corner line.
type Calendar struct {
	Today Today
	Days  []Day  // tomorrow onwards
	Bhg   string // "bhg: 12"; "" until fetched
}

type Today struct {
	Kids   calendar.KidsStatus
	Dinner string       // several joined with " · "
	Notes  []string     // birthdays ("Kai Lützenkirchen fyller 45"), then full-day events
	Events []TodayEvent // timed events in start order
}

type TodayEvent struct {
	Time  string // "12:00"; "" for an event that started on an earlier day
	Title string
	Hytta string // the cabin's forecast, "(5°, 20 % regn)", on events that mention hytta
	Faded bool
}

type Day struct {
	Kids    calendar.KidsStatus
	Label   string // "Tirsdag"; with the date, "Mandag 2.", once the name would repeat
	Dinner  string
	Weather string     // "5–12° · regn", "5–12°", ""
	Events  []DayEvent // birthdays ("Kai 45 år"), full-day titles, then "11:00 PT (Lena)"
}

type DayEvent struct {
	Text  string
	Faded bool
}

// rainWordFrom is the daily light-rain probability (percent) from which a
// coming day says "regn".
const rainWordFrom = 50

// BuildCalendar lays out today and the next days-1 days.
func BuildCalendar(cal calendar.Snapshot, daily, dailyHytta map[string]weather.Daily, now time.Time, days int) Calendar {
	now = now.In(tz.Oslo)
	out := Calendar{Today: buildToday(cal, dailyHytta, now)}
	if cal.BhgDaysLeft != nil {
		out.Bhg = "bhg: " + strconv.Itoa(*cal.BhgDaysLeft)
	}
	for i := 1; i < days; i++ {
		out.Days = append(out.Days, buildDay(cal, daily, now.AddDate(0, 0, i), i))
	}
	return out
}

func buildToday(cal calendar.Snapshot, dailyHytta map[string]weather.Daily, date time.Time) Today {
	t := Today{Kids: cal.KidsStatusForDate(date), Dinner: dinners(cal, date)}
	for _, b := range cal.BirthdaysForDate(date) {
		text := b.DisplayTitle
		if name, age, ok := b.Birthday(date.Year()); ok {
			text = name + " fyller " + strconv.Itoa(age)
		}
		t.Notes = append(t.Notes, text)
	}
	hytta, hasHytta := dailyHytta[date.Format(time.DateOnly)]
	suffix := func(e calendar.EnrichedEvent) string {
		if !hasHytta || !strings.Contains(strings.ToLower(e.Title), "hytta") {
			return ""
		}
		return "(" + Deg(hytta.MaxTemp) + ", " + Pct(hytta.LightRainProbability) + " regn)"
	}
	for _, e := range eventsByStart(cal, date) {
		if e.FullDay {
			note := e.DisplayTitle
			if s := suffix(e); s != "" {
				note += " " + s
			}
			t.Notes = append(t.Notes, note)
			continue
		}
		t.Events = append(t.Events, TodayEvent{
			Time:  e.DisplayTime.Start,
			Title: e.DisplayTitle,
			Hytta: suffix(e),
			Faded: e.Faded,
		})
	}
	return t
}

func buildDay(cal calendar.Snapshot, daily map[string]weather.Daily, date time.Time, i int) Day {
	d := Day{
		Kids:   cal.KidsStatusForDate(date),
		Label:  Capitalize(weekdays[date.Weekday()]),
		Dinner: dinners(cal, date),
	}
	if i >= 7 {
		d.Label += " " + strconv.Itoa(date.Day()) + "."
	}
	if f, ok := daily[date.Format(time.DateOnly)]; ok {
		d.Weather = DegRange(f.MinTemp, f.MaxTemp)
		if f.LightRainProbability >= rainWordFrom {
			d.Weather += " · regn"
		}
	}
	for _, b := range cal.BirthdaysForDate(date) {
		text := b.DisplayTitle
		if name, age, ok := b.Birthday(date.Year()); ok {
			first, _, _ := strings.Cut(name, " ")
			text = first + " " + strconv.Itoa(age) + " år"
		}
		d.Events = append(d.Events, DayEvent{Text: text})
	}
	events := eventsByStart(cal, date)
	for _, e := range events {
		if e.FullDay {
			d.Events = append(d.Events, DayEvent{Text: e.DisplayTitle, Faded: e.Faded})
		}
	}
	for _, e := range events {
		if e.FullDay {
			continue
		}
		text := e.DisplayTitle
		if e.DisplayTime.Start != "" {
			// A no-break space: the line may wrap inside an appointment, but
			// never between the time and the title.
			text = e.DisplayTime.Start + " " + text
		}
		d.Events = append(d.Events, DayEvent{Text: text, Faded: e.Faded})
	}
	return d
}

func dinners(cal calendar.Snapshot, date time.Time) string {
	var names []string
	for _, e := range cal.DinnersForDate(date) {
		names = append(names, e.DisplayTitle)
	}
	return strings.Join(names, " · ")
}

// eventsByStart is the day's events in start order; the snapshot's order
// is not guaranteed.
func eventsByStart(cal calendar.Snapshot, date time.Time) []calendar.EnrichedEvent {
	events := cal.EventsForDate(date)
	slices.SortStableFunc(events, func(a, b calendar.EnrichedEvent) int { return a.Start.Compare(b.Start) })
	return events
}
