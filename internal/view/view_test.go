package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
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

// midNov is exactly half of a 30-day month, so a straight-line use of the
// quota is half of it.
var midNov = time.Date(2026, 11, 16, 0, 0, 0, 0, tz.Oslo)

// The percentage is how far ahead (+) or behind (−) a straight-line use of
// the Norgespris quota the month is, so heading for spot prices shows early.
func TestBuildPowerBehindPace(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, AccumulatedConsumption: 14.25, EffectivePrice: 0.5, MonthlyConsumption: 2000, Cap: 5000}
	want := Power{Label: "Strøm hjemme", HasData: true, Power: "0,0 kW", Today: "14 kWh", Detail: "0,50 kr · −20 %"}
	if got := BuildPower(d, tibber.Home, midNov); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	d.CurrentPower = -400
	if got := BuildPower(d, tibber.Home, midNov).Power; got != "−0,4 kW" {
		t.Errorf("producing: got %q", got)
	}
}

func TestBuildPowerAheadOfPace(t *testing.T) {
	// 600 of 1000 kWh is under the cap, but ahead of the 500 a straight line allows.
	d := tibber.PowerData{Timestamp: ten, EffectivePrice: 0.5, MonthlyConsumption: 600, Cap: 1000}
	p := BuildPower(d, tibber.Cabin, midNov)
	if p.Label != "Hytta" || p.Detail != "0,50 kr · +20 %" {
		t.Errorf("got %+v", p)
	}
}

func TestBuildPowerOnPace(t *testing.T) {
	for _, kWh := range []float64{2500, 2498} { // exactly on pace, and just under: never "−0"
		d := tibber.PowerData{Timestamp: ten, EffectivePrice: 0.5, MonthlyConsumption: kWh, Cap: 5000}
		if got := BuildPower(d, tibber.Home, midNov).Detail; got != "0,50 kr · 0 %" {
			t.Errorf("%v kWh: got %q", kWh, got)
		}
	}
}

func TestBuildPowerWithoutCap(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, CurrentPrice: 0.22}
	if got := BuildPower(d, tibber.Cabin, midNov).Detail; got != "0,22 kr" {
		t.Errorf("got %q", got)
	}
}

func TestBuildPowerNoData(t *testing.T) {
	want := Power{Label: "Strøm hjemme"}
	if got := BuildPower(tibber.PowerData{}, tibber.Home, midNov); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
