package power

import (
	"math"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/spotpris"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func ptr(f float64) *float64 { return &f }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func nov(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, tz.Oslo) }

// testPower has Norgespris 0.5 with a cap of 100 kWh at home and 10 at the
// cabin.
func testPower() *Power {
	p := New(Config{})
	p.norgespris = Norgespris{Prices: []PricePeriod{{Price: 0.5}}, HomeCap: 100, CabinCap: 10}
	return p
}

// hourly makes n hours of kwh each from start.
func hourly(start time.Time, n int, kwh float64) []Usage {
	var out []Usage
	for i := range n {
		out = append(out, Usage{Start: start.Add(time.Duration(i) * time.Hour), Consumption: kwh})
	}
	return out
}

// pricesFor gives every hour of the days from first to last the price total.
func pricesFor(p *Power, first, last time.Time, total float64) {
	for d := tz.StartOfDay(first); !d.After(last); d = d.AddDate(0, 0, 1) {
		var day []spotpris.Price
		for h := d; h.Before(d.AddDate(0, 0, 1)); h = h.Add(time.Hour) {
			day = append(day, spotpris.Price{Start: h, Total: total})
		}
		p.storePrices(d, day)
	}
}

func TestAccumulatedCostCrossesCap(t *testing.T) {
	n := Norgespris{Prices: []PricePeriod{{Price: 0.5}}, HomeCap: 10, CabinCap: 5}
	hours := []Usage{{Start: nov(1, 0, 0), Consumption: 6}, {Start: nov(1, 1, 0), Consumption: 6}, {Start: nov(1, 2, 0), Consumption: 1}}
	spot := func(h time.Time) float64 { return map[int]float64{0: 2, 1: 2, 2: 3}[h.Hour()] }
	// 6 subsidized: 3.0; 4 subsidized + 2 at spot: 2.0 + 4.0; over the cap: 3.0
	if got := n.AccumulatedCost(Home, hours, spot, 0); !near(got, 12) {
		t.Errorf("got %v, want 12", got)
	}
	// With 8 kWh used before, only 2 are left under the cap.
	if got := n.AccumulatedCost(Home, hours[:1], spot, 8); !near(got, 2*0.5+4*2) {
		t.Errorf("got %v, want 9", got)
	}
}

func TestNorgesprisPriceChangesOnNewYear2027(t *testing.T) {
	n := defaultNorgespris
	oslo := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, tz.Oslo) }
	cases := []struct {
		at   time.Time
		want float64
	}{
		{oslo(2025, 10, 1, 0, 0), 0.50},
		{oslo(2026, 12, 31, 23, 59), 0.50},
		{oslo(2027, 1, 1, 0, 0), 0.5625}, // 45 øre + 25 % VAT
		{oslo(2027, 6, 1, 12, 0), 0.5625},
	}
	for _, c := range cases {
		if got := n.PriceAt(c.at); !near(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.at, got, c.want)
		}
		if got := n.EffectivePrice(Home, c.at, 100, 3); !near(got, c.want) {
			t.Errorf("%s under the cap: got %v, want %v", c.at, got, c.want)
		}
	}
	if got := n.EffectivePrice(Home, oslo(2027, 1, 5, 12, 0), 5000, 3); got != 3 {
		t.Errorf("at the cap the spot price applies, got %v", got)
	}

	// Each hour is priced at the rate in effect when it was used.
	hours := []Usage{{Start: oslo(2026, 12, 31, 23, 0), Consumption: 2}, {Start: oslo(2027, 1, 1, 0, 0), Consumption: 2}}
	if got := n.AccumulatedCost(Home, hours, func(time.Time) float64 { return 1 }, 0); !near(got, 2*0.5+2*0.5625) {
		t.Errorf("cost across the change: got %v", got)
	}
}

// Without a live feed, today is the known hours, and the cost goes hour by
// hour across the cap.
func TestDataFromHoursWithoutLive(t *testing.T) {
	p := testPower()
	now := nov(26, 13, 40)
	hours := append(hourly(nov(25, 0, 0), 24, 4), hourly(nov(26, 0, 0), 13, 1)...)
	p.storeHours(Home, hours, now)
	pricesFor(p, nov(25, 0, 0), nov(26, 0, 0), 2)

	d := p.Data(Home, now)
	if !near(d.AccumulatedConsumption, 13) || !d.TodayUntil.Equal(nov(26, 13, 0)) {
		t.Errorf("today: %v kWh until %v", d.AccumulatedConsumption, d.TodayUntil)
	}
	if !near(d.MonthlyConsumption, 109) || !d.MonthKnown {
		t.Errorf("month: %v kWh, known %v", d.MonthlyConsumption, d.MonthKnown)
	}
	// 96 kWh before today, so 4 of today's are under the cap and 9 at spot.
	if want := 4*0.5 + 9*2.0; !near(d.AccumulatedCost, want) {
		t.Errorf("today's cost %v, want %v", d.AccumulatedCost, want)
	}
	if want := 96*0.5 + 4*0.5 + 9*2.0; !near(d.MonthlyCost, want) {
		t.Errorf("month's cost %v, want %v", d.MonthlyCost, want)
	}
	if d.CurrentPrice != 2 || d.EffectivePrice != 2 || d.Cap != 100 {
		t.Errorf("price %v, effective %v, cap %v", d.CurrentPrice, d.EffectivePrice, d.Cap)
	}
}

// A current Tibber feed gives today's total; its cost splits at the cap with
// the price now.
func TestDataUsesLiveWhenCurrent(t *testing.T) {
	p := testPower()
	p.norgespris.CabinCap = 100
	now := nov(26, 13, 40)
	p.storeHours(Cabin, append(hourly(nov(25, 0, 0), 24, 4), hourly(nov(26, 0, 0), 13, 1)...), now)
	pricesFor(p, nov(25, 0, 0), nov(26, 0, 0), 2)
	p.measure(Cabin, tibber.Measurement{Power: 800, AccumulatedConsumption: 15}, now.Add(-2*time.Minute))

	d := p.Data(Cabin, now)
	if !d.IsLive(now) || !near(d.AccumulatedConsumption, 15) || !d.TodayUntil.IsZero() {
		t.Errorf("today: live %v, %v kWh until %v", d.IsLive(now), d.AccumulatedConsumption, d.TodayUntil)
	}
	want := 4*0.5 + 11*2.0
	if !near(d.AccumulatedCost, want) || !near(d.MonthlyConsumption, 111) {
		t.Errorf("cost %v (want %v), month %v", d.AccumulatedCost, want, d.MonthlyConsumption)
	}
	if !near(d.MonthlyCost, 96*0.5+want) {
		t.Errorf("month's cost %v", d.MonthlyCost)
	}

	// Eleven minutes later the feed counts as gone, and the hours take over.
	if d := p.Data(Cabin, now.Add(11*time.Minute)); d.IsLive(now.Add(11*time.Minute)) || !near(d.AccumulatedConsumption, 13) {
		t.Errorf("stale: %v kWh", d.AccumulatedConsumption)
	}
}

// After midnight on the 1st, last month's hours and prices don't count, and
// the month isn't known until this month's first fetch.
func TestDataIgnoresLastMonth(t *testing.T) {
	p := testPower()
	p.storeHours(Home, hourly(nov(30, 0, 0), 24, 4), nov(30, 23, 50))
	now := time.Date(2026, 12, 1, 0, 30, 0, 0, tz.Oslo)
	d := p.Data(Home, now)
	if d.MonthlyConsumption != 0 || d.MonthKnown || d.AccumulatedConsumption != 0 || !d.TodayUntil.IsZero() {
		t.Errorf("got %+v", d)
	}
	if got := p.Details(Home, now).Days; len(got) != 0 {
		t.Errorf("days %+v", got)
	}
}

// Over the cap, an hour without a price counts nothing at spot.
func TestDataHourWithoutPrice(t *testing.T) {
	p := testPower()
	now := nov(26, 13, 40)
	p.storeHours(Home, hourly(nov(26, 0, 0), 2, 60), now)
	d := p.Data(Home, now)
	if !near(d.AccumulatedCost, 100*0.5) || d.CurrentPrice != 0 {
		t.Errorf("cost %v, price %v", d.AccumulatedCost, d.CurrentPrice)
	}
}

// When the lists don't reach now, the last known price stays.
func TestCurrentPriceKeepsLastKnown(t *testing.T) {
	p := testPower()
	pricesFor(p, nov(25, 0, 0), nov(25, 0, 0), 1.4)
	if got := p.Data(Home, nov(26, 9, 0)).CurrentPrice; got != 1.4 {
		t.Errorf("got %v, want 1.4", got)
	}
	pricesFor(p, nov(26, 0, 0), nov(26, 0, 0), 2)
	if got := p.Data(Home, nov(26, 9, 0)).CurrentPrice; got != 2 {
		t.Errorf("got %v, want today's 2", got)
	}
	if got := p.Data(Home, nov(24, 9, 0)).CurrentPrice; got != 0 {
		t.Errorf("before any price: got %v", got)
	}
}

// Today and tomorrow first, then back to the 1st.
func TestWantedDays(t *testing.T) {
	days := func(now time.Time) (int, time.Time, time.Time) {
		w := wantedDays(now)
		return len(w), w[0], w[len(w)-1]
	}
	if n, first, last := days(nov(26, 12, 59)); n != 26 || !first.Equal(nov(26, 0, 0)) || !last.Equal(nov(1, 0, 0)) {
		t.Errorf("before 13: %d days, %v … %v", n, first, last)
	}
	if w := wantedDays(nov(26, 13, 0)); len(w) != 27 || !w[1].Equal(nov(27, 0, 0)) || !w[2].Equal(nov(25, 0, 0)) {
		t.Errorf("from 13: %v", w)
	}
	dec1 := time.Date(2026, 12, 1, 0, 10, 0, 0, tz.Oslo)
	if n, first, _ := days(dec1); n != 1 || !first.Equal(tz.StartOfDay(dec1)) {
		t.Errorf("on the 1st: %d days from %v", n, first)
	}
}

// Details has the history's hours today, filled in by the feed's, the days
// before today, and today's and tomorrow's prices.
func TestDetailsMergesHistoryAndLive(t *testing.T) {
	p := testPower()
	now := nov(26, 11, 30)
	p.storeHours(Cabin, append(hourly(nov(25, 0, 0), 24, 1), hourly(nov(26, 0, 0), 9, 0.5)...), now)
	pricesFor(p, nov(20, 0, 0), nov(27, 0, 0), 1)
	for _, m := range []struct {
		at  time.Time
		acc float64
	}{{nov(26, 8, 59).Add(30 * time.Second), 4}, {nov(26, 9, 0), 4.5}, {nov(26, 9, 59).Add(30 * time.Second), 5}, {nov(26, 10, 0), 5.1}, {nov(26, 10, 59).Add(30 * time.Second), 6}, {nov(26, 11, 0), 6.2}, {nov(26, 11, 29), 7}} {
		p.measure(Cabin, tibber.Measurement{AccumulatedConsumption: m.acc}, m.at)
	}

	d := p.Details(Cabin, now)
	if len(d.Hours) != 12 || !near(d.Hours[9].Consumption, 1) || !near(d.Hours[11].Consumption, 1) {
		t.Errorf("hours %+v", d.Hours)
	}
	if len(d.Days) != 1 || d.Days[0].Consumption != 24 {
		t.Errorf("days %+v", d.Days)
	}
	if len(d.Prices) != 48 || !d.Prices[0].Start.Equal(nov(26, 0, 0)) {
		t.Errorf("%d prices from %v", len(d.Prices), d.Prices[0].Start)
	}
	if d.Norgespris != 0.5 || !near(d.AccumulatedConsumption, 7) {
		t.Errorf("got %+v", d.PowerData)
	}
}

func TestMeasureNetsProduction(t *testing.T) {
	p := testPower()
	now := nov(26, 12, 0)
	p.measure(Cabin, tibber.Measurement{AccumulatedConsumption: 4, AccumulatedProduction: 1.5, MaxPowerProduction: ptr(900)}, now)
	d := p.Data(Cabin, now)
	if d.AccumulatedProduction != 1.5 || d.AccumulatedConsumption != 2.5 || d.MaxPowerProduction != 900 {
		t.Errorf("got %+v", d)
	}
}

func TestCabinShowsProductionAsNegativePower(t *testing.T) {
	p := testPower()
	now := nov(26, 12, 0)
	p.measure(Cabin, tibber.Measurement{Power: 0, PowerProduction: ptr(1500)}, now)
	if got := p.Data(Cabin, now).CurrentPower; got != -1500 {
		t.Errorf("got %v, want -1500", got)
	}
	// A frame without production keeps using the last known value.
	p.measure(Cabin, tibber.Measurement{Power: 0}, now)
	if got := p.Data(Cabin, now).CurrentPower; got != -1500 {
		t.Errorf("got %v, want -1500", got)
	}
}
