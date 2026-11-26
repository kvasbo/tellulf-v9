package power

import (
	"encoding/hex"
	"testing"
	"time"
)

// A frame from the home's meter with active power import 11686 W.
var list1, _ = hex.DecodeString("7ea02a410883130413e6e7000f40000000000101020309060100010700ff0600002da602020f00161bd0967e")

const list1Watts = 11686.0

// The HAN reader gives the power now, and fills today with the hours Elvia
// doesn't have yet by counting the energy between readings.
func TestHANFillsTheHoursElviaLacks(t *testing.T) {
	p := testPower()
	p.storeHours(Home, hourly(nov(26, 0, 0), 12, 1), nov(26, 13, 0)) // 00–11
	pricesFor(p, nov(26, 0, 0), nov(26, 0, 0), 2)
	start := nov(26, 12, 59).Add(30 * time.Second)
	now := nov(26, 13, 30)
	for at := start; !at.After(now); at = at.Add(30 * time.Second) {
		p.han(list1, at)
	}

	d := p.Data(Home, now)
	if !d.IsLive(now) || d.CurrentPower != list1Watts || d.MinPower != list1Watts || d.MaxPower != list1Watts {
		t.Fatalf("live: %v, power %v, min %v, max %v", d.IsLive(now), d.CurrentPower, d.MinPower, d.MaxPower)
	}
	// Hour 13 runs from the last reading before 13:00 (12:59:30) to now.
	hour13 := list1Watts * 30.5 / 60 / 1000
	if want := 12 + hour13; !near(d.AccumulatedConsumption, want) || !d.TodayUntil.IsZero() {
		t.Errorf("today %v kWh, want %v", d.AccumulatedConsumption, want)
	}
	if want := (12 + hour13) * 1000 / 13.5; !near(d.AveragePower, want) {
		t.Errorf("average %v W, want %v", d.AveragePower, want)
	}
	if want := (12 + hour13) * 0.5; !near(d.AccumulatedCost, want) {
		t.Errorf("cost %v, want %v (all under the cap)", d.AccumulatedCost, want)
	}
	hours := p.Details(Home, now).Hours
	// Hour 12 is there from the first reading (12:59:30) on: nothing yet.
	if len(hours) != 14 || hours[12].Consumption != 0 || !hours[13].Start.Equal(nov(26, 13, 0)) || !near(hours[13].Consumption, hour13) {
		t.Errorf("hours %+v", hours)
	}

	// Once the reader is quiet for over ten minutes, the hours are all there is.
	later := now.Add(11 * time.Minute)
	if d := p.Data(Home, later); d.IsLive(later) || d.AccumulatedConsumption != 12 || !d.TodayUntil.Equal(nov(26, 12, 0)) {
		t.Errorf("stale: %v kWh until %v", d.AccumulatedConsumption, d.TodayUntil)
	}
}

// Energy isn't counted across a gap in the readings, and the count starts
// over at midnight.
func TestHANCountsOnlyCloseReadings(t *testing.T) {
	p := testPower()
	p.han(list1, nov(26, 10, 0))
	p.han(list1, nov(26, 10, 5)) // five minutes: not counted
	if got := p.hanHome.today; got != 0 {
		t.Errorf("after a gap: %v kWh", got)
	}
	p.han(list1, nov(26, 10, 5).Add(10*time.Second))
	if got, want := p.hanHome.today, list1Watts*10/3600/1000; !near(got, want) {
		t.Errorf("got %v kWh, want %v", got, want)
	}
	p.han(list1, nov(27, 0, 0).Add(5*time.Second))
	if got := p.hanHome.today; got != 0 {
		t.Errorf("after midnight: %v kWh", got)
	}
}

func TestHANSkipsOtherMessages(t *testing.T) {
	p := testPower()
	p.han([]byte(`{"status":{"rssi":-65}}`), nov(26, 10, 0))
	damaged := append([]byte(nil), list1...)
	damaged[30] ^= 1
	p.han(damaged, nov(26, 10, 0))
	if d := p.Data(Home, nov(26, 10, 0)); !d.Timestamp.IsZero() {
		t.Errorf("got %+v", d)
	}
}

// After a gap in the readings the HAN count hasn't grown, so the hour the
// reader comes back in is still marked, and the hour before keeps its count.
func TestHANMarksHoursAfterAGap(t *testing.T) {
	p := testPower()
	p.han(list1, nov(26, 13, 58))
	p.han(list1, nov(26, 13, 58).Add(10*time.Second))
	p.han(list1, nov(26, 14, 10)) // back after twelve minutes
	p.han(list1, nov(26, 14, 10).Add(10*time.Second))
	hours := p.Details(Home, nov(26, 14, 11)).Hours
	ten := list1Watts * 10 / 3600 / 1000
	// Hour 13 has what was counted before the gap, hour 14 what came after.
	if len(hours) != 2 || !near(hours[0].Consumption, ten) || !hours[1].Start.Equal(nov(26, 14, 0)) || !near(hours[1].Consumption, ten) {
		t.Errorf("hours %+v", hours)
	}
}
