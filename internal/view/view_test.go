package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
)

// ten is 10:00 in Oslo on an autumn day.
var ten = time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

func departures(now time.Time, minutes ...int) []entur.Train {
	var out []entur.Train
	for _, m := range minutes {
		out = append(out, entur.Train{Time: now.Add(time.Duration(m) * time.Minute), Destination: "Bergkrystallen", Realtime: true})
	}
	return out
}

func TestBuildTrainsSkipsDepartedAndImminent(t *testing.T) {
	got := BuildTrains(departures(ten, -3, 1, 2, 5, 9), ten)
	want := []Train{{Time: "10:02"}, {Time: "10:05"}}
	if !slices.Equal(got.Departures, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildTrainsDots(t *testing.T) {
	trains := departures(ten, 5, 10)
	trains[0].Delayed = true
	trains[1].Realtime = false
	got := BuildTrains(trains, ten)
	if got.Departures[0].Dot != "red" || got.Departures[1].Dot != "grey" {
		t.Errorf("dots = %q %q, want red grey", got.Departures[0].Dot, got.Departures[1].Dot)
	}
}

func TestBuildTrainsNoneReachable(t *testing.T) {
	for _, trains := range [][]entur.Train{nil, departures(ten, -1, 0, 1)} {
		got := BuildTrains(trains, ten)
		if len(got.Departures) != 0 {
			t.Errorf("got %+v, want nothing", got)
		}
	}
}

// Under the cap the cell shows the month's use and how much of the quota it
// has taken, and no price: every kWh costs the same fixed Norgespris.
func TestBuildPowerUnderCap(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, EffectivePrice: 0.5, MonthlyConsumption: 2345, Cap: 5000}
	want := Power{Label: "Strøm hjemme", HasData: true, Power: "0,0 kW", Usage: "2345 kWh · 47 %"}
	if got := BuildPower(d, tibber.Home); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	d.CurrentPower = -400
	if got := BuildPower(d, tibber.Home).Power; got != "−0,4 kW" {
		t.Errorf("producing: got %q", got)
	}
}

// Once the quota is used up, kWh cost spot price again, so the price shows.
func TestBuildPowerAtAndOverCap(t *testing.T) {
	for kWh, usage := range map[float64]string{1000: "1000 kWh · 100 %", 1234: "1234 kWh · 123 %"} {
		d := tibber.PowerData{Timestamp: ten, EffectivePrice: 1.234, MonthlyConsumption: kWh, Cap: 1000}
		p := BuildPower(d, tibber.Cabin)
		if p.Label != "Hytta" || p.Usage != usage || p.Price != "1,23 kr" {
			t.Errorf("%v kWh: got %+v", kWh, p)
		}
	}
}

// Without a cap there is no quota share, and the price always matters.
func TestBuildPowerWithoutCap(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, CurrentPrice: 0.22, MonthlyConsumption: 812.4}
	p := BuildPower(d, tibber.Cabin)
	if p.Usage != "812 kWh" || p.Price != "0,22 kr" {
		t.Errorf("got %+v", p)
	}
}

func TestBuildPowerNoData(t *testing.T) {
	want := Power{Label: "Strøm hjemme"}
	if got := BuildPower(tibber.PowerData{}, tibber.Home); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
