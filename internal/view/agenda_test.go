package view

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/claude"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// plain reads text with ordinary spaces, so expectations stay readable.
func plain(s string) string { return strings.NewReplacer("\u202f", " ", "\u00a0", " ").Replace(s) }

func agendaEvents(d AgendaDay) []string {
	var out []string
	for _, e := range d.Events {
		out = append(out, plain(e.Time+"|"+e.Title+"|"+e.Detail))
	}
	return out
}

func TestAgendaLabelsAndWeather(t *testing.T) {
	daily := map[string]weather.Daily{
		"2026-10-26": {MinTemp: 3.4, MaxTemp: 8.2, LightRainProbability: 0},
		"2026-10-27": {MinTemp: 4, MaxTemp: 9, LightRainProbability: 60},
	}
	a := BuildAgenda(calendar.Snapshot{}, daily, nil, nil, calNow, 14)
	if len(a.Days) != 14 {
		t.Fatalf("got %d days, want 14", len(a.Days))
	}
	if d := a.Days[0]; !d.Today || d.Label != "I dag" || d.Weather != "3–8°" {
		t.Errorf("today: %+v", d)
	}
	if d := a.Days[1]; d.Today || d.Label != "Tirsdag 27. oktober" || plain(d.Weather) != "4–9° · 60 % regn" {
		t.Errorf("tomorrow: %+v", d)
	}
	if d := a.Days[6]; d.Label != "Søndag 1. november" || d.Weather != "" {
		t.Errorf("day 6: %+v", d)
	}
}

// Every appointment has its end, its place (the part before the first
// comma) and its full title; one that started earlier has no start time.
func TestAgendaEventDetails(t *testing.T) {
	tue := mon.AddDate(0, 0, 1)
	long := "Software-møte om ny arkitektur for betalingsløsningen og migrering av kunder"
	pt := timed("PT (Lena)", mon.Add(11*time.Hour), time.Hour)
	pt.Location = "SATS Majorstuen, Kirkeveien 64, 0364 Oslo"
	cal := calendar.Snapshot{Events: []calendar.Event{
		pt,
		timed(long, mon.Add(14*time.Hour), 2*time.Hour),
		// Monday 16:00 to Wednesday 18:00.
		timed("Seminar", mon.Add(16*time.Hour), 2*24*time.Hour+2*time.Hour),
		// Monday 20:00 to the Tuesday a week later.
		timed("Lang reise", mon.Add(20*time.Hour), 8*24*time.Hour),
	}}
	a := BuildAgenda(cal, nil, nil, nil, calNow, 3)
	want := []string{
		"11:00|PT (Lena)|til 12:00 · SATS Majorstuen",
		"14:00|" + long + "|til 16:00",
		"16:00|Seminar|til onsdag 18:00",
		"20:00|Lang reise|til tirsdag 3. november 20:00",
	}
	if got := agendaEvents(a.Days[0]); !slices.Equal(got, want) {
		t.Errorf("monday:\n got %q\nwant %q", got, want)
	}
	want = []string{"|Seminar|til onsdag 18:00", "|Lang reise|til tirsdag 3. november 20:00"}
	if got := agendaEvents(a.Days[1]); !slices.Equal(got, want) {
		t.Errorf("tuesday %s:\n got %q\nwant %q", tue.Format("2.1"), got, want)
	}
	// From Wednesday, next Tuesday is less than a week away.
	want = []string{"|Seminar|til 18:00", "|Lang reise|til tirsdag 20:00"}
	if got := agendaEvents(a.Days[2]); !slices.Equal(got, want) {
		t.Errorf("wednesday:\n got %q\nwant %q", got, want)
	}
}

// Notes: birthdays with the full name and age, then full-day events, which
// say until when they last. The cabin's weather follows what mentions it.
func TestAgendaNotesAndHytta(t *testing.T) {
	hytta := map[string]weather.Daily{"2026-10-26": {MaxTemp: 5.4, LightRainProbability: 20}}
	cal := calendar.Snapshot{
		Birthdays: []calendar.Event{fullDay("Kai Lützenkirchen 1981", mon)},
		Events: []calendar.Event{
			{Title: "Høstferie", Start: mon, End: mon.AddDate(0, 0, 4), FullDay: true},
			fullDay("Turdag E", mon),
			timed("Til hytta", mon.Add(17*time.Hour), time.Hour),
		},
		Dinners: []calendar.Event{fullDay("Pannekaker", mon), fullDay("Is", mon)},
	}
	d := BuildAgenda(cal, nil, hytta, nil, calNow, 1).Days[0]
	var notes []string
	for _, n := range d.Notes {
		notes = append(notes, plain(n.Text))
	}
	wantNotes := []string{"Kai Lützenkirchen fyller 45", "Høstferie · til og med fredag", "Turdag E"}
	if !slices.Equal(notes, wantNotes) {
		t.Errorf("notes:\n got %q\nwant %q", notes, wantNotes)
	}
	if d.Dinner != "Pannekaker · Is" {
		t.Errorf("dinner %q", d.Dinner)
	}
	if got := agendaEvents(d); !slices.Equal(got, []string{"17:00|Til hytta|til 18:00 · 5°, 20 % regn"}) {
		t.Errorf("events %q", got)
	}
}

// The kids being away fades the family calendar's events, as on the
// overview.
func TestAgendaFadesWhenKidsAway(t *testing.T) {
	e := timed("Fotball", mon.Add(17*time.Hour), time.Hour)
	e.Source = calendar.Felles
	d := BuildAgenda(calendar.Snapshot{Events: []calendar.Event{e}}, nil, nil, nil, calNow, 1).Days[0]
	if len(d.Events) != 1 || !d.Events[0].Faded || d.Kids != calendar.KidsAway {
		t.Errorf("got %+v", d)
	}
}

func TestShortPlace(t *testing.T) {
	cases := map[string]string{
		"Storgata 1, 0155 Oslo":       "Storgata 1",
		"  Teams  ":                   "Teams",
		"Hos Tore\nGata 2":            "Hos Tore",
		"":                            "",
		"Oslo Spektrum, Sonja Henies": "Oslo Spektrum",
	}
	for in, want := range cases {
		if got := shortPlace(in); got != want {
			t.Errorf("shortPlace(%q) = %q, want %q", in, got, want)
		}
	}
}

// Claude's note gets a line of its own under the detail line. A title the
// overview would cut gets Claude's short title; others stay in full.
func TestAgendaUsesClaudesTitles(t *testing.T) {
	long := timed("Invitasjon: Kvartalsgjennomgang økonomi og drift Q4 – ledergruppen", mon.Add(12*time.Hour), time.Hour)
	medium := timed("Foreldremøte 3. trinn Slemdal skole, aula", mon.Add(18*time.Hour), time.Hour)
	fotball := timed("Fotball", mon.Add(17*time.Hour), time.Hour)
	fotball.Description = "Oppmøte 16:45. Husk drikkeflaske!"
	ferie := calendar.Event{Title: "Høstferie", Description: "Skolen er stengt", Start: mon, End: mon.AddDate(0, 0, 4), FullDay: true}
	titles := claude.Titles{
		claude.TitleKey(long):    {Short: "Kvartalsgjennomgang Q4", Note: "Les rapporten før møtet"},
		claude.TitleKey(medium):  {Short: "Foreldremøte 3. trinn"},
		claude.TitleKey(fotball): {Note: "Ta med drikkeflaske"},
		claude.TitleKey(ferie):   {Note: "Skolen er stengt"},
	}
	d := BuildAgenda(calendar.Snapshot{Events: []calendar.Event{long, medium, fotball, ferie}}, nil, nil, titles, calNow, 1).Days[0]
	var got []string
	for _, e := range d.Events {
		got = append(got, e.Title+"|"+e.Note)
	}
	want := []string{"Kvartalsgjennomgang Q4|Les rapporten før møtet", "Fotball|Ta med drikkeflaske", "Foreldremøte 3. trinn Slemdal skole, aula|"}
	if !slices.Equal(got, want) {
		t.Errorf("events:\n got %q\nwant %q", got, want)
	}
	if len(d.Notes) != 1 || plain(d.Notes[0].Text) != "Høstferie · til og med fredag · Skolen er stengt" {
		t.Errorf("notes %+v", d.Notes)
	}
}
