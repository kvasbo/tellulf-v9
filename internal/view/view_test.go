package view

import (
	"slices"
	"strings"
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

func TestBuildPowerHome(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, EffectivePrice: 0.5, MonthlyConsumption: 1350, Cap: 5000}
	want := Power{Label: "Strøm hjemme", HasData: true, Power: "0,0 kW", Detail: "0,50 kr · 27 %"}
	if got := BuildPower(d, tibber.Home); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	d.CurrentPower = -400
	if got := BuildPower(d, tibber.Home).Power; got != "−0,4 kW" {
		t.Errorf("producing: got %q", got)
	}
}

func TestBuildPowerAboveCap(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, EffectivePrice: 1.2, MonthlyConsumption: 1080, Cap: 1000}
	p := BuildPower(d, tibber.Cabin)
	if p.Label != "Hytta" || !strings.HasSuffix(p.Detail, "108 %") {
		t.Errorf("got %+v", p)
	}
}

func TestBuildPowerWithoutCap(t *testing.T) {
	d := tibber.PowerData{Timestamp: ten, CurrentPrice: 0.22}
	if got := BuildPower(d, tibber.Cabin).Detail; got != "0,22 kr" {
		t.Errorf("got %q", got)
	}
}

func TestBuildPowerNoData(t *testing.T) {
	want := Power{Label: "Strøm hjemme"}
	if got := BuildPower(tibber.PowerData{}, tibber.Home); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
