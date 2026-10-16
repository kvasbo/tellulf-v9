package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

func TestBuildTrainsSkipsDeparted(t *testing.T) {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	var trains []entur.Train
	for _, m := range []int{-3, -1, 2, 5, 9, 12, 15} {
		trains = append(trains, entur.Train{Time: now.Add(time.Duration(m) * time.Minute), Realtime: true})
	}
	got := BuildTrains(trains, now)
	if len(got) != 4 || got[0].Time != "10:02" || got[3].Time != "10:12" {
		t.Errorf("got %+v, want the four from 10:02", got)
	}
}

func TestBuildCalendarGivesHyttaEventsTheCabinForecast(t *testing.T) {
	fri := time.Date(2026, 10, 16, 0, 0, 0, 0, tz.Oslo)
	cal := calendar.Snapshot{Events: []calendar.Event{
		// Timed, over a weekend
		{Title: "Hytta", Start: fri.Add(13 * time.Hour), End: fri.AddDate(0, 0, 2).Add(15 * time.Hour)},
		{Title: "Middag", Start: fri.Add(18 * time.Hour), End: fri.Add(20 * time.Hour)},
		// Full day
		{Title: "Til hytta", Start: fri.AddDate(0, 0, 3), End: fri.AddDate(0, 0, 3), FullDay: true},
	}}
	hytta := map[string]weather.Daily{}
	for i := range 4 {
		hytta[fri.AddDate(0, 0, i).Format(time.DateOnly)] = weather.Daily{MaxTemp: float64(10 + i), LightRainProbability: 20}
	}

	c := BuildCalendar(cal, nil, hytta, fri.Add(9*time.Hour), 4)
	want := [][]string{{"10", ""}, {"11"}, {"12"}, {"13"}} // "" = no forecast (Middag)
	for i, d := range c.Days {
		var got []string
		for _, e := range d.Events {
			if e.Hytta == nil {
				got = append(got, "")
				continue
			}
			got = append(got, e.Hytta.Temperature)
		}
		if !slices.Equal(got, want[i]) {
			t.Errorf("day %d: got %q, want %q", i, got, want[i])
		}
	}
}
