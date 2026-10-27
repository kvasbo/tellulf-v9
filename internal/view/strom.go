package view

import (
	"math"
	"strconv"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Strom is one place's column on the power page: the power now with the
// day's average, peak and low beside it, then three rows of a text block and
// a chart: today by the hour, the month day by day against the Norgespris
// quota, and the spot price today and tomorrow.
type Strom struct {
	Label   string // "Hjemme", "Hytta"
	HasData bool   // false until the live feed has delivered
	Now     string // "2,4" (kW, which the template adds)
	Line1   string // "Snitt 1,1 kW · topp 6,2 kW"
	Line2   string // "Lavest 0,3 kW i dag", plus "· sol topp 3,4 kW"
	Rows    []Row  // I DAG, the month, SPOTPRIS
}

var stromLabels = map[tibber.Place]string{tibber.Home: "Hjemme", tibber.Cabin: "Hytta"}

// minHourScale keeps a quiet day from filling the hourly chart: the scale
// goes to at least this many kWh.
const minHourScale = 1.0

func BuildStrom(d tibber.Details, place tibber.Place, now time.Time) Strom {
	s := Strom{Label: stromLabels[place], HasData: !d.Timestamp.IsZero()}
	if !s.HasData {
		return s
	}
	now = now.In(tz.Oslo)
	s.Now = nb(Fixed(d.CurrentPower/1000, 1))
	s.Line1 = "Snitt " + KW(d.AveragePower) + " · topp " + KW(d.MaxPower)
	s.Line2 = "Lavest " + KW(d.MinPower) + " i dag"
	if d.MaxPowerProduction > 0 {
		s.Line2 += " · sol topp " + KW(d.MaxPowerProduction)
	}
	s.Rows = []Row{todayRow(d, now), monthRow(d, now), priceRow(d, now)}
	return s
}

// todayRow: today's net consumption and cost, and the consumption hour by
// hour.
func todayRow(d tibber.Details, now time.Time) Row {
	r := Row{Label: "I dag", Value: KWh(d.AccumulatedConsumption), Details: []string{Kr(d.AccumulatedCost)}}
	if d.AccumulatedProduction > 0 {
		r.Details = append(r.Details, "sol "+KWh(d.AccumulatedProduction))
	}
	// Keyed by Unix time: Tibber's times carry their own zone, and equal
	// instants in different zones are different map keys.
	used := map[int64]float64{}
	for _, h := range d.Hours {
		used[h.Start.Unix()] = h.Consumption
	}
	var slots []slot
	top := minHourScale
	for _, h := range hoursOf(tz.StartOfDay(now)) {
		v, ok := used[h.Unix()]
		top = max(top, v)
		slots = append(slots, slot{value: v, known: ok, class: nowClass(h, now), tick: hourTick(h)})
	}
	c := barChart(slots, 0, top, chartWidth)
	r.Chart = &c
	return r
}

// monthRow: the month's consumption, its share of the quota, the pace
// against it, and the cost, with the consumption added up day by day
// against a straight-line use of the quota.
func monthRow(d tibber.Details, now time.Time) Row {
	r := Row{Label: Capitalize(months[now.Month()-1]), Value: KWh(d.MonthlyConsumption)}
	if d.Cap > 0 {
		r.Details = append(r.Details,
			Pct(d.MonthlyConsumption/d.Cap*100)+" av kvoten",
			paceVsQuota(d.MonthlyConsumption, d.Cap, now)+" mot tempo")
	}
	r.Details = append(r.Details, Kr(d.MonthlyCost))

	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
	n := first.AddDate(0, 1, -1).Day()
	used := map[int]float64{}
	for _, u := range d.Days {
		if u.Start.In(tz.Oslo).Month() == now.Month() {
			used[u.Start.In(tz.Oslo).Day()] = u.Consumption
		}
	}
	slots := make([]slot, n)
	total := 0.0
	for day := 1; day <= n; day++ {
		s := &slots[day-1]
		if (day-1)%7 == 0 {
			s.tick = strconv.Itoa(day) + "."
		}
		switch {
		case day < now.Day():
			if v, ok := used[day]; ok {
				total += v
				s.value, s.known = total, true
			}
		case day == now.Day():
			// The month's total, so today's bar matches the number beside it.
			s.value, s.known, s.class = d.MonthlyConsumption, true, "now"
		}
	}
	top := max(d.Cap, d.MonthlyConsumption, total, 1)
	c := barChart(slots, 0, top, chartWidth)
	if d.Cap > 0 {
		capY := Fixed(yFor(d.Cap, 0, top), 1)
		c.Lines = append(c.Lines,
			hline(yFor(d.Cap, 0, top), chartWidth, "base"),
			Line{X1: "0", Y1: Fixed(plotHeight, 0), X2: Fixed(chartWidth, 0), Y2: capY, Kind: "ref"})
	}
	r.Chart = &c
	return r
}

// priceRow: the spot price now, today's range and the Norgespris, with the
// price hour by hour today and tomorrow.
func priceRow(d tibber.Details, now time.Time) Row {
	r := Row{Label: "Spotpris"}
	if len(d.Prices) == 0 {
		return r
	}
	price := map[int64]float64{} // by Unix time, as in todayRow
	for _, p := range d.Prices {
		price[p.Start.Unix()] = p.Total
	}
	today := tz.StartOfDay(now)
	tomorrow := today.AddDate(0, 0, 1)
	lo, hi := 0.0, d.Norgespris
	dayLo, dayHi := math.Inf(1), math.Inf(-1)
	var slots []slot
	for _, h := range append(hoursOf(today), hoursOf(tomorrow)...) {
		v, ok := price[h.Unix()]
		s := slot{value: v, known: ok, class: nowClass(h, now), tick: hourTick(h)}
		if h.Equal(tomorrow) {
			s.tick = shortWeekday(tomorrow)
		}
		if ok {
			lo, hi = min(lo, v), max(hi, v)
			if tz.SameDay(h, today) {
				dayLo, dayHi = min(dayLo, v), max(dayHi, v)
			}
		}
		slots = append(slots, s)
	}
	r.Value = Kr(d.CurrentPrice)
	if dayLo <= dayHi {
		r.Details = append(r.Details, nb(Fixed(dayLo, 2))+"–"+Kr(dayHi)+" i dag")
	}
	if d.Norgespris > 0 {
		r.Details = append(r.Details, "Norgespris "+Kr(d.Norgespris))
	}
	c := barChart(slots, lo, hi, chartWidth)
	if d.Norgespris > 0 {
		c.Lines = append(c.Lines, hline(yFor(d.Norgespris, lo, hi), chartWidth, "ref"))
	}
	r.Chart = &c
	return r
}

// hoursOf lists the starts of the hours of the day beginning at midnight
// day: 24, or 23 and 25 when the clocks change.
func hoursOf(day time.Time) []time.Time {
	var out []time.Time
	next := day.AddDate(0, 0, 1)
	for h := day; h.Before(next); h = h.Add(time.Hour) {
		out = append(out, h)
	}
	return out
}

// nowClass marks the slot of the hour now.
func nowClass(hour, now time.Time) string {
	if hour.Equal(now.Truncate(time.Hour)) {
		return "now"
	}
	return ""
}

// shortWeekday is the weekday's first three letters, "ons", for an axis.
func shortWeekday(t time.Time) string {
	return string([]rune(weekdays[t.In(tz.Oslo).Weekday()])[:3])
}

// hourTick labels every sixth hour of the clock: "00", "06", "12", "18".
func hourTick(h time.Time) string {
	h = h.In(tz.Oslo)
	if h.Hour()%6 != 0 || h.Minute() != 0 {
		return ""
	}
	return h.Format("15")
}

// paceVsQuota is how far ahead (+) or behind (−) a straight-line use of cap
// over the month the consumption is, in whole percent: "+20 %", "−67 %",
// "0 %". Ahead means on course to pass the cap, and pay spot price, before
// the month ends.
func paceVsQuota(consumption, cap float64, now time.Time) string {
	now = now.In(tz.Oslo)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
	monthEnd := monthStart.AddDate(0, 1, 0)
	gone := float64(now.Sub(monthStart)) / float64(monthEnd.Sub(monthStart))
	expected := math.Max(gone*cap, 1) // at midnight on the 1st nothing is expected yet
	diff := tz.Round((consumption - expected) / expected * 100)
	if diff > 0 {
		return "+" + Pct(diff)
	}
	return Pct(diff)
}
