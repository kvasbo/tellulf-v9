package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Agenda is the calendar page: today and the coming days, each laid out
// like the I DAG block on the overview, with every detail the calendar has.
type Agenda struct {
	Days []AgendaDay
}

type AgendaDay struct {
	Today   bool
	Label   string // "I dag", "Onsdag 28. oktober"
	Weather string // "5–12° · 60 % regn", "5–12°"; "" without a forecast
	Kids    calendar.KidsStatus
	Dinner  string // several joined with " · "
	Notes   []Note // birthdays ("Kai Lützenkirchen fyller 45"), then full-day events
	Events  []AgendaEvent
}

type Note struct {
	Text  string
	Faded bool
}

type AgendaEvent struct {
	Time   string // "12:00"; "" for an event that started on an earlier day
	Title  string // in full
	Detail string // "til 13:30 · Storgata 1 · 5°, 20 % regn"
	Faded  bool
}

// BuildAgenda lays out today and the next days-1 days.
func BuildAgenda(cal calendar.Snapshot, daily, dailyHytta map[string]weather.Daily, now time.Time, days int) Agenda {
	now = now.In(tz.Oslo)
	var a Agenda
	for i := range days {
		a.Days = append(a.Days, buildAgendaDay(cal, daily, dailyHytta, now.AddDate(0, 0, i), i == 0))
	}
	return a
}

func buildAgendaDay(cal calendar.Snapshot, daily, dailyHytta map[string]weather.Daily, date time.Time, today bool) AgendaDay {
	d := AgendaDay{
		Today:  today,
		Label:  longDate(date),
		Kids:   cal.KidsStatusForDate(date),
		Dinner: dinners(cal, date),
	}
	if today {
		d.Label = "I dag"
	}
	key := date.Format(time.DateOnly)
	if f, ok := daily[key]; ok {
		d.Weather = DegRange(f.MinTemp, f.MaxTemp)
		if f.LightRainProbability > 0 {
			d.Weather += " · " + Pct(f.LightRainProbability) + " regn"
		}
	}
	hytta := ""
	if f, ok := dailyHytta[key]; ok {
		hytta = Deg(f.MaxTemp) + ", " + Pct(f.LightRainProbability) + " regn"
	}
	mentionsHytta := func(e calendar.EnrichedEvent) bool {
		return hytta != "" && strings.Contains(strings.ToLower(e.Title), "hytta")
	}

	for _, b := range cal.BirthdaysForDate(date) {
		text := b.DisplayTitle
		if name, age, ok := b.Birthday(date.Year()); ok {
			text = name + " fyller " + strconv.Itoa(age)
		}
		d.Notes = append(d.Notes, Note{Text: text})
	}
	events := eventsByStart(cal, date)
	for _, e := range events {
		if !e.FullDay {
			continue
		}
		text := fullTitle(e)
		if !tz.SameDay(e.End, date) {
			text += " · til og med " + dayRef(e.End, date)
		}
		if mentionsHytta(e) {
			text += " (" + hytta + ")"
		}
		d.Notes = append(d.Notes, Note{Text: text, Faded: e.Faded})
	}
	for _, e := range events {
		if e.FullDay {
			continue
		}
		var detail []string
		end := e.End.In(tz.Oslo).Format("15:04")
		if !tz.SameDay(e.End, date) {
			end = dayRef(e.End, date) + " " + end
		}
		detail = append(detail, "til "+end)
		if place := shortPlace(e.Location); place != "" {
			detail = append(detail, place)
		}
		if mentionsHytta(e) {
			detail = append(detail, hytta)
		}
		d.Events = append(d.Events, AgendaEvent{
			Time:   e.DisplayTime.Start,
			Title:  fullTitle(e),
			Detail: strings.Join(detail, " · "),
			Faded:  e.Faded,
		})
	}
	return d
}

// fullTitle is the event's title without the overview's 50-character cut.
func fullTitle(e calendar.EnrichedEvent) string {
	if strings.TrimSpace(e.Title) == "" {
		return e.DisplayTitle // "(uten tittel)"
	}
	return e.Title
}

// shortPlace keeps the first part of a location, which names the place;
// the rest is usually its address: "Storgata 1, 0155 Oslo" → "Storgata 1".
func shortPlace(loc string) string {
	first, _, _ := strings.Cut(loc, ",")
	first, _, _ = strings.Cut(first, "\n")
	return strings.TrimSpace(first)
}
