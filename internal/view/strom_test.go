package view

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func oslo(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, tz.Oslo) }

// stromDetails is a home at 11:30 on 27 October, behind the quota's pace.
func stromDetails() tibber.Details {
	d := tibber.Details{
		PowerData: tibber.PowerData{
			Timestamp:              oslo(27, 11, 30),
			CurrentPower:           2440,
			AveragePower:           1130,
			MaxPower:               6210,
			MinPower:               320,
			AccumulatedConsumption: 14.24,
			AccumulatedCost:        7.12,
			MonthlyConsumption:     2345,
			MonthlyCost:            1172.5,
			CurrentPrice:           0.571,
			Cap:                    5000,
		},
		Norgespris: 0.5,
	}
	for h := range 11 {
		d.Hours = append(d.Hours, tibber.Usage{Start: oslo(27, h, 0), Consumption: float64(h) / 4})
	}
	for day := 1; day < 27; day++ {
		d.Days = append(d.Days, tibber.Usage{Start: oslo(day, 0, 0), Consumption: 85})
	}
	for h := range 24 {
		d.Prices = append(d.Prices, tibber.PricePoint{Start: oslo(27, h, 0), Total: 0.19 + float64(h)/60})
	}
	return d
}

func tickTexts(ticks []Tick) []string {
	var out []string
	for _, t := range ticks {
		out = append(out, t.Text)
	}
	return out
}

func nowBars(c *Chart) int {
	n := 0
	for _, b := range c.Bars {
		if b.Class == "now" {
			n++
		}
	}
	return n
}

func TestBuildStromNoData(t *testing.T) {
	got := BuildStrom(tibber.Details{}, tibber.Cabin, oslo(27, 11, 30))
	if got.Label != "Hytta" || got.HasData || got.Now != "" || got.Rows != nil {
		t.Errorf("got %+v, want just the label", got)
	}
}

func TestBuildStromText(t *testing.T) {
	got := BuildStrom(stromDetails(), tibber.Home, oslo(27, 11, 30))
	if got.Label != "Hjemme" || got.Now != "2,4" ||
		got.Line1 != "Snitt 1,1 kW · topp 6,2 kW" || got.Line2 != "Lavest 0,3 kW i dag" {
		t.Errorf("head: got %+v", got)
	}
	want := []struct {
		label, value string
		details      []string
	}{
		{"I dag", "14 kWh", []string{"7,12 kr"}},
		{"Oktober", "2345 kWh", []string{"47 % av kvoten", "−45 % mot tempo", "1172,50 kr"}},
		{"Spotpris", "0,57 kr", []string{"0,19–0,57 kr i dag", "Norgespris 0,50 kr"}},
	}
	if len(got.Rows) != len(want) {
		t.Fatalf("rows: got %+v", got.Rows)
	}
	for i, w := range want {
		r := got.Rows[i]
		if r.Label != w.label || r.Value != w.value || !slices.Equal(r.Details, w.details) {
			t.Errorf("row %d: got %q %q %q, want %q %q %q", i, r.Label, r.Value, r.Details, w.label, w.value, w.details)
		}
	}
}

// The cabin's production shows as negative power, and as its own line.
func TestBuildStromProduction(t *testing.T) {
	d := stromDetails()
	d.CurrentPower = -1840
	d.MaxPowerProduction = 3400
	d.AccumulatedProduction = 3.1
	got := BuildStrom(d, tibber.Cabin, oslo(27, 11, 30))
	if got.Now != "−1,8" || got.Line2 != "Lavest 0,3 kW i dag · sol topp 3,4 kW" ||
		!slices.Equal(got.Rows[0].Details, []string{"7,12 kr", "sol 3,1 kWh"}) {
		t.Errorf("got %q, %q, %q", got.Now, got.Line2, got.Rows[0].Details)
	}
}

// Today by the hour: one place per hour, bars where there is data, the
// hour now in the accent, and every sixth hour labelled.
func TestTodayChart(t *testing.T) {
	c := BuildStrom(stromDetails(), tibber.Home, oslo(27, 11, 30)).Rows[0].Chart
	if len(c.Bars) != 11 || nowBars(c) != 0 {
		t.Errorf("bars: got %d, %d now; want 11 and none (11:00 has no data yet)", len(c.Bars), nowBars(c))
	}
	if got := tickTexts(c.Ticks); !slices.Equal(got, []string{"00", "06", "12", "18"}) {
		t.Errorf("ticks %q", got)
	}
	// The tallest hour (2.5 kWh) fills the plot.
	if c.Bars[10].Y != "0.0" || c.Bars[10].H != strconv.Itoa(plotHeight)+".0" {
		t.Errorf("tallest bar %+v", c.Bars[10])
	}

	d := stromDetails()
	d.Hours = append(d.Hours, tibber.Usage{Start: oslo(27, 11, 0), Consumption: 0.4})
	c = BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows[0].Chart
	if nowBars(c) != 1 || c.Bars[11].Class != "now" {
		t.Errorf("the hour now should be the accent bar: %+v", c.Bars)
	}
}

// Tibber's times carry their own zone ("+01:00"); the same instant in
// another zone is still the same hour.
func TestChartsMatchHoursAcrossZones(t *testing.T) {
	d := stromDetails()
	tibberZone := time.FixedZone("", 3600)
	for i := range d.Hours {
		d.Hours[i].Start = d.Hours[i].Start.In(tibberZone)
	}
	for i := range d.Prices {
		d.Prices[i].Start = d.Prices[i].Start.In(tibberZone)
	}
	rows := BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows
	if n := len(rows[0].Chart.Bars); n != 11 {
		t.Errorf("hours: %d bars, want 11", n)
	}
	if n := len(rows[2].Chart.Bars); n != 24 {
		t.Errorf("prices: %d bars, want 24", n)
	}
}

// The day the clocks go back has 25 hours, and the chart a place for each.
func TestTodayChartOnDSTDay(t *testing.T) {
	d := stromDetails()
	d.Hours = nil
	c := BuildStrom(d, tibber.Home, oslo(25, 12, 0)).Rows[0].Chart
	if got := tickTexts(c.Ticks); !slices.Equal(got, []string{"00", "06", "12", "18"}) {
		t.Errorf("ticks %q", got)
	}
	if x := c.Ticks[1].X; x != Fixed(float64(chartWidth)/25*7, 1) {
		t.Errorf("06 sits at %s, want the eighth of 25 places", x)
	}
}

// The month adds up day by day against a straight line to the quota at
// the month's end, with the quota itself as a limit line.
func TestMonthChart(t *testing.T) {
	c := BuildStrom(stromDetails(), tibber.Home, oslo(27, 11, 30)).Rows[1].Chart
	if len(c.Bars) != 27 || c.Bars[26].Class != "now" {
		t.Fatalf("bars: got %d, last now %v", len(c.Bars), c.Bars[len(c.Bars)-1].Class)
	}
	if got := tickTexts(c.Ticks); !slices.Equal(got, []string{"1.", "8.", "15.", "22.", "29."}) {
		t.Errorf("ticks %q", got)
	}
	// Scaled to the quota: 2345 of 5000 kWh is 47 % of the plot.
	if want := Fixed(plotHeight*2345.0/5000, 1); c.Bars[26].H != want {
		t.Errorf("today's bar is %s high, want %s", c.Bars[26].H, want)
	}
	// Day 1 is 85 kWh, day 26 is 26 × 85.
	if c.Bars[0].H != Fixed(plotHeight*85.0/5000, 1) || c.Bars[25].H != Fixed(plotHeight*26*85.0/5000, 1) {
		t.Errorf("cumulative bars: %+v, %+v", c.Bars[0], c.Bars[25])
	}
	kinds := map[string]Line{}
	for _, l := range c.Lines {
		kinds[l.Kind+l.Y2] = l
	}
	if _, ok := kinds["base0.0"]; !ok {
		t.Errorf("the quota line should sit at the top: %+v", c.Lines)
	}
	if l, ok := kinds["ref0.0"]; !ok || l.X1 != "0" || l.Y1 != Fixed(plotHeight, 0) {
		t.Errorf("the pace line should run from the bottom left to the quota: %+v", c.Lines)
	}

	// Past the quota the scale follows the consumption.
	d := stromDetails()
	d.MonthlyConsumption = 6000
	c = BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows[1].Chart
	if c.Bars[26].Y != "0.0" {
		t.Errorf("past the quota today's bar should reach the top: %+v", c.Bars[26])
	}
}

// Prices: today and tomorrow, 48 places, the hour now in the accent, and
// the Norgespris as a reference line. Tomorrow's first tick is its weekday.
func TestPriceChart(t *testing.T) {
	c := BuildStrom(stromDetails(), tibber.Home, oslo(27, 11, 30)).Rows[2].Chart
	if len(c.Bars) != 24 || nowBars(c) != 1 || c.Bars[11].Class != "now" {
		t.Errorf("bars: got %d, now %d", len(c.Bars), nowBars(c))
	}
	if got := tickTexts(c.Ticks); !slices.Equal(got, []string{"00", "06", "12", "18", "ons", "06", "12", "18"}) {
		t.Errorf("ticks %q", got)
	}
	var ref []Line
	for _, l := range c.Lines {
		if l.Kind == "ref" {
			ref = append(ref, l)
		}
	}
	// hi is today's top price, 0.19 + 23/60.
	if want := Fixed(yFor(0.5, 0, 0.19+23.0/60), 1); len(ref) != 1 || ref[0].Y1 != want {
		t.Errorf("Norgespris line %+v, want at y %s", ref, want)
	}
}

// A negative price hangs below a zero line.
func TestPriceChartNegative(t *testing.T) {
	d := stromDetails()
	d.Prices[3].Total = -0.1
	c := BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows[2].Chart
	hi := 0.19 + 23.0/60
	zero := Fixed(yFor(0, -0.1, hi), 1)
	if b := c.Bars[3]; b.Y != zero || b.H != Fixed(yFor(-0.1, -0.1, hi)-yFor(0, -0.1, hi), 1) {
		t.Errorf("negative bar %+v, want from the zero line at %s", b, zero)
	}
	if got := BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows[2].Details[0]; got != "−0,10–0,57 kr i dag" {
		t.Errorf("range %q", got)
	}
}

func TestPriceRowWithoutPrices(t *testing.T) {
	d := stromDetails()
	d.Prices = nil
	r := BuildStrom(d, tibber.Home, oslo(27, 11, 30)).Rows[2]
	if r.Label != "Spotpris" || r.Value != "" || r.Details != nil || r.Chart != nil {
		t.Errorf("got %+v, want just the label", r)
	}
}

// Pace compares the month's use with a straight-line use of the quota.
func TestPaceVsQuota(t *testing.T) {
	cases := []struct {
		used float64
		at   time.Time
		want string
	}{
		{2500, time.Date(2026, 11, 16, 0, 0, 0, 0, tz.Oslo), "0 %"},   // half the month, half the quota
		{3000, time.Date(2026, 11, 16, 0, 0, 0, 0, tz.Oslo), "+20 %"}, // ahead
		{0, time.Date(2026, 11, 1, 0, 0, 0, 0, tz.Oslo), "−100 %"},    // the first minute
	}
	for _, c := range cases {
		if got := paceVsQuota(c.used, 5000, c.at); got != c.want {
			t.Errorf("%v at %s: got %q, want %q", c.used, c.at, got, c.want)
		}
	}
}

func TestBuildClock(t *testing.T) {
	got := BuildClock(time.Date(2026, 10, 27, 8, 5, 0, 0, time.UTC))
	want := Clock{Time: "9:05", Date: "Tirsdag 27. oktober", Week: "uke 44"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
