package view

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

var (
	mon    = time.Date(2026, 10, 26, 0, 0, 0, 0, tz.Oslo)
	calNow = mon.Add(11 * time.Hour)
)

func fullDay(title string, d time.Time) calendar.Event {
	return calendar.Event{Title: title, Start: d, End: d, FullDay: true}
}

func timed(title string, start time.Time, d time.Duration) calendar.Event {
	return calendar.Event{Title: title, Start: start, End: start.Add(d)}
}

func texts(events []DayEvent) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Text)
	}
	return out
}

func TestTodaySplitsDinnerNotesAndEvents(t *testing.T) {
	snap := calendar.Snapshot{
		Dinners:   []calendar.Event{fullDay("Pannekaker", mon)},
		Birthdays: []calendar.Event{fullDay("Kai Lützenkirchen 1981", mon)},
		Events: []calendar.Event{
			fullDay("Turdag E", mon),
			timed("Agnes på turn", mon.Add(17*time.Hour), 90*time.Minute),
			timed("Passesta: Audun og Tore", mon.Add(12*time.Hour), time.Hour),
		},
	}
	c := BuildCalendar(snap, nil, nil, calNow, 2)
	if c.Today.Dinner != "Pannekaker" {
		t.Errorf("dinner = %q", c.Today.Dinner)
	}
	if want := []string{"Kai Lützenkirchen fyller 45", "Turdag E"}; !slices.Equal(c.Today.Notes, want) {
		t.Errorf("notes = %q, want %q", c.Today.Notes, want)
	}
	want := []TodayEvent{{Time: "12:00", Title: "Passesta: Audun og Tore"}, {Time: "17:00", Title: "Agnes på turn"}}
	if !slices.Equal(c.Today.Events, want) {
		t.Errorf("events = %+v, want %+v", c.Today.Events, want)
	}
	if len(c.Days) != 1 {
		t.Errorf("got %d coming days, want 1", len(c.Days))
	}
}

func TestTodayMultiDayEventHasNoTime(t *testing.T) {
	sun := mon.AddDate(0, 0, -1)
	snap := calendar.Snapshot{Events: []calendar.Event{
		{Title: "Hyttetur", Start: sun.Add(20 * time.Hour), End: mon.AddDate(0, 0, 1).Add(10 * time.Hour)},
	}}
	c := BuildCalendar(snap, nil, nil, calNow, 1)
	want := []TodayEvent{{Time: "", Title: "Hyttetur"}}
	if !slices.Equal(c.Today.Events, want) {
		t.Errorf("events = %+v, want %+v", c.Today.Events, want)
	}
}

func TestDayLabelsAddDateFromDaySeven(t *testing.T) {
	c := BuildCalendar(calendar.Snapshot{}, nil, nil, calNow, 9)
	var got []string
	for _, d := range c.Days {
		got = append(got, d.Label)
	}
	want := []string{"Tirsdag", "Onsdag", "Torsdag", "Fredag", "Lørdag", "Søndag", "Mandag 2.", "Tirsdag 3."}
	if !slices.Equal(got, want) {
		t.Errorf("labels = %q, want %q", got, want)
	}
}

func TestDayWeatherAndRainWord(t *testing.T) {
	daily := map[string]weather.Daily{
		mon.AddDate(0, 0, 1).Format(time.DateOnly): {MinTemp: 4.6, MaxTemp: 12.2, LightRainProbability: 50},
		mon.AddDate(0, 0, 2).Format(time.DateOnly): {MinTemp: 5, MaxTemp: 11, LightRainProbability: 40},
	}
	c := BuildCalendar(calendar.Snapshot{}, daily, nil, calNow, 4)
	want := []string{"5–12° · regn", "5–11°", ""}
	for i, w := range want {
		if got := c.Days[i].Weather; got != w {
			t.Errorf("day %d weather = %q, want %q", i, got, w)
		}
	}
}

func TestDayWithoutForecast(t *testing.T) {
	c := BuildCalendar(calendar.Snapshot{}, map[string]weather.Daily{}, nil, calNow, 3)
	for i, d := range c.Days {
		if d.Weather != "" || strings.Contains(d.Weather, "·") {
			t.Errorf("day %d weather = %q, want empty", i, d.Weather)
		}
	}
}

func TestDayEventsLine(t *testing.T) {
	tue := mon.AddDate(0, 0, 1)
	snap := calendar.Snapshot{
		Dinners:   []calendar.Event{fullDay("Lasagne", tue), fullDay("Is", tue)},
		Birthdays: []calendar.Event{fullDay("Kai Lützenkirchen 1981", tue)},
		Events: []calendar.Event{
			{Title: "PT (Lena)", Start: tue.Add(11 * time.Hour), End: tue.Add(12 * time.Hour), Source: calendar.Felles},
			fullDay("Turdag E", tue),
		},
	}
	c := BuildCalendar(snap, nil, nil, calNow, 2)
	d := c.Days[0]
	if d.Dinner != "Lasagne · Is" {
		t.Errorf("dinner = %q", d.Dinner)
	}
	if want := []string{"Kai 45 år", "Turdag E", "11:00 PT (Lena)"}; !slices.Equal(texts(d.Events), want) {
		t.Errorf("events = %q, want %q", texts(d.Events), want)
	}
	if !d.Events[2].Faded || d.Events[1].Faded {
		t.Errorf("faded flags = %v %v, want the Felles event faded only", d.Events[1].Faded, d.Events[2].Faded)
	}
}

func TestHyttaSuffixOnlyToday(t *testing.T) {
	tue := mon.AddDate(0, 0, 1)
	snap := calendar.Snapshot{Events: []calendar.Event{
		timed("Hytta", mon.Add(13*time.Hour), 2*time.Hour),
		timed("Hytta", tue.Add(13*time.Hour), 2*time.Hour),
	}}
	hytta := map[string]weather.Daily{
		mon.Format(time.DateOnly): {MaxTemp: 5.2, LightRainProbability: 20},
		tue.Format(time.DateOnly): {MaxTemp: 5.2, LightRainProbability: 20},
	}
	c := BuildCalendar(snap, nil, hytta, calNow, 2)
	if got := c.Today.Events[0].Hytta; got != "(5°, 20 % regn)" {
		t.Errorf("today hytta = %q", got)
	}
	if got := c.Days[0].Events[0].Text; got != "13:00 Hytta" {
		t.Errorf("tomorrow text = %q", got)
	}
}

func TestBhgCorner(t *testing.T) {
	n := 12
	if got := BuildCalendar(calendar.Snapshot{BhgDaysLeft: &n}, nil, nil, calNow, 1).Bhg; got != "bhg: 12" {
		t.Errorf("bhg = %q", got)
	}
	if got := BuildCalendar(calendar.Snapshot{}, nil, nil, calNow, 1).Bhg; got != "" {
		t.Errorf("bhg = %q, want empty", got)
	}
}
