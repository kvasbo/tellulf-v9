package view

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/sun"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Vaer is the weather page: on the left the weather now, as on the
// overview, and the next 48 hours as three rows of a text block and a
// chart (temperature, precipitation, wind); on the right the sun and the
// coming days for Oslo and the cabin.
type Vaer struct {
	Now    Weather // the temperature now and the two sentences; Columns unused
	Sensor string  // "Fuktighet 81 % · 1013 hPa"; "" without those sensors
	Rows   []Row
	Sun    Row // label, rise–set, day length and its change; no chart
	Days   []VaerDay
}

// VaerDay is one coming day in the table: Oslo and the cabin side by side.
type VaerDay struct {
	Label       string // "Onsdag"; with the date, "Mandag 2.", once the name would repeat
	Oslo, Hytta DayWeather
}

type DayWeather struct {
	Temp string // "4–9°"; "" without a forecast for the day
	Rain string // "30 %"
	Wet  bool   // the rain is likely enough (rainWordFrom) to colour
}

const (
	vaerHours = 48 // the charts' span
	vaerDays  = 12 // the table's rows, from tomorrow
	// The hours the text blocks sum up: the next day, so "Mest kl. 15"
	// can't mean two different days.
	summaryHours = 24
	// Precipitation per hour (mm) for the three shades, as on the overview.
	moderateRain = 0.5
	heavyRain    = 2.0
)

func BuildVaer(r smarthouse.Readings, hours []weather.Hourly, daily, dailyHytta map[string]weather.Daily, now time.Time) Vaer {
	now = now.In(tz.Oslo)
	v := Vaer{Now: BuildWeather(r, hours, now), Sun: sunRow(now)}
	v.Now.Columns = nil
	var sensor []string
	if r.HumOut != nil {
		sensor = append(sensor, "Fuktighet "+Pct(*r.HumOut))
	}
	if r.Pressure != nil {
		sensor = append(sensor, "lufttrykk "+nb(Num(tz.Round(*r.Pressure)))+nbsp+"hPa")
	}
	v.Sensor = Capitalize(strings.Join(sensor, " · "))

	if len(hours) > vaerHours {
		hours = hours[:vaerHours]
	}
	if len(hours) > 0 {
		v.Rows = []Row{tempRow(hours), rainRow(hours), windRow(hours)}
	}
	for i := 1; i <= vaerDays; i++ {
		date := now.AddDate(0, 0, i)
		d := VaerDay{Label: Capitalize(weekdays[date.Weekday()])}
		if i >= 7 {
			d.Label += " " + strconv.Itoa(date.Day()) + "."
		}
		key := date.Format(time.DateOnly)
		d.Oslo, d.Hytta = dayWeather(daily, key), dayWeather(dailyHytta, key)
		v.Days = append(v.Days, d)
	}
	return v
}

func dayWeather(daily map[string]weather.Daily, key string) DayWeather {
	f, ok := daily[key]
	if !ok {
		return DayWeather{}
	}
	return DayWeather{
		Temp: DegRange(f.MinTemp, f.MaxTemp),
		Rain: Pct(f.LightRainProbability),
		Wet:  f.LightRainProbability >= rainWordFrom,
	}
}

// tempRow: the next day's range and when it is warmest and coldest, and the
// temperature as a curve.
func tempRow(hours []weather.Hourly) Row {
	r := Row{Label: "Temperatur"}
	lo, hi := math.Inf(1), math.Inf(-1)
	var loAt, hiAt int
	slots := hourSlots(hours)
	for i, h := range hours {
		if h.AirTemperature == nil {
			continue
		}
		t := *h.AirTemperature
		slots[i].value, slots[i].known = t, true
		if i >= summaryHours {
			continue
		}
		if t < lo {
			lo, loAt = t, i
		}
		if t > hi {
			hi, hiAt = t, i
		}
	}
	if lo > hi {
		return r // no temperatures at all
	}
	r.Value = DegRange(lo, hi)
	r.Details = []string{"Varmest " + atHour(hours, hiAt), "Kaldest " + atHour(hours, loAt)}

	// Scale to the whole span with a little room, so the curve never
	// touches the edges.
	min48, max48 := math.Inf(1), math.Inf(-1)
	for _, s := range slots {
		if s.known {
			min48, max48 = math.Min(min48, s.value), math.Max(max48, s.value)
		}
	}
	bottom, top := math.Floor(min48)-2, math.Ceil(max48)+2
	c := axisChart(slots, wideChart)
	c.Lines = append(c.Lines, midnights(hours)...)
	if bottom < 0 && top > 0 {
		c.Lines = append(c.Lines, hline(yFor(0, bottom, top), wideChart, "base"))
	}
	c.Curves = []Curve{curve(slots, bottom, top, wideChart, "temp")}
	r.Chart = &c
	return r
}

// rainRow: the next day's total, when it starts or stops, and the wettest
// hour, with the precipitation per hour in the three rain shades.
func rainRow(hours []weather.Hourly) Row {
	r := Row{Label: "Nedbør"}
	slots := hourSlots(hours)
	total, peak, peakAt := 0.0, 0.0, 0
	top := heavyRain // so drizzle stays small
	for i, h := range hours {
		mm := h.PrecipitationAmount
		top = math.Max(top, mm)
		if mm >= wet {
			slots[i].value, slots[i].known, slots[i].class = mm, true, rainClass(mm)
		}
		if i < summaryHours {
			total += mm
			if mm > peak {
				peak, peakAt = mm, i
			}
		}
	}
	if total < wet {
		r.Value = "Opphold"
	} else {
		r.Value = MM(total)
	}
	if p := rainPart(hours); p != "" {
		r.Details = append(r.Details, Capitalize(p))
	}
	if peak >= wet {
		r.Details = append(r.Details, "Mest "+MM(peak)+" "+atHour(hours, peakAt))
	}
	c := barChart(slots, 0, top, wideChart)
	c.Lines = append(c.Lines, midnights(hours)...)
	r.Chart = &c
	return r
}

// windRow: the wind now, its gusts and direction, and the strongest hour,
// with the wind per hour as bars and the gusts as a curve over them.
func windRow(hours []weather.Hourly) Row {
	r := Row{Label: "Vind"}
	slots, gusts := hourSlots(hours), hourSlots(hours)
	top, strongest, strongestAt := 5.0, -1.0, 0
	for i, h := range hours {
		if h.WindSpeed != nil {
			slots[i].value, slots[i].known = *h.WindSpeed, true
			top = math.Max(top, *h.WindSpeed)
			if i < summaryHours && *h.WindSpeed > strongest {
				strongest, strongestAt = *h.WindSpeed, i
			}
		}
		if h.WindSpeedOfGust != nil {
			gusts[i].value, gusts[i].known = *h.WindSpeedOfGust, true
			top = math.Max(top, *h.WindSpeedOfGust)
		}
	}
	now := hours[0]
	if now.WindSpeed != nil {
		r.Value = MS(*now.WindSpeed)
	}
	if now.WindSpeedOfGust != nil {
		r.Details = append(r.Details, "Kast "+MS(*now.WindSpeedOfGust))
	}
	if now.WindFromDirection != nil {
		r.Details = append(r.Details, "Fra "+compass(*now.WindFromDirection))
	}
	if strongest >= 0 && strongestAt > 0 {
		r.Details = append(r.Details, "Mest "+MS(strongest)+" "+atHour(hours, strongestAt))
	}
	c := barChart(slots, 0, top, wideChart)
	c.Lines = append(c.Lines, midnights(hours)...)
	if g := curve(gusts, 0, top, wideChart, "gust"); g.Points != "" {
		c.Curves = []Curve{g}
	}
	r.Chart = &c
	return r
}

// hourSlots is one empty slot per forecast hour, with every sixth hour of
// the clock labelled and midnight as the new day's weekday.
func hourSlots(hours []weather.Hourly) []slot {
	slots := make([]slot, len(hours))
	for i, h := range hours {
		slots[i].tick = hourTick(h.Time)
		if h.Hour == 0 && slots[i].tick != "" {
			slots[i].tick = shortWeekday(h.Time)
		}
	}
	return slots
}

// midnights are faint lines where a new day starts.
func midnights(hours []weather.Hourly) []Line {
	step := float64(wideChart) / float64(len(hours))
	var out []Line
	for i, h := range hours {
		if i > 0 && h.Hour == 0 && h.Time.Minute() == 0 {
			x := Fixed(float64(i)*step-step*(1-barShare)/2, 1)
			out = append(out, Line{X1: x, Y1: "0", X2: x, Y2: strconv.Itoa(plotHeight), Kind: "base"})
		}
	}
	return out
}

// atHour names hour i of the forecast: "nå" for the first, else "kl. 15".
func atHour(hours []weather.Hourly, i int) string {
	if i == 0 {
		return "nå"
	}
	return "kl. " + hours[i].Time.Format("15")
}

func rainClass(mm float64) string {
	switch {
	case mm < moderateRain:
		return "rain-1"
	case mm < heavyRain:
		return "rain-2"
	}
	return "rain-3"
}

// compass names the direction the wind comes from: "nord", "sørvest".
func compass(deg float64) string {
	names := [...]string{"nord", "nordøst", "øst", "sørøst", "sør", "sørvest", "vest", "nordvest"}
	return names[int(math.Mod(deg+22.5+360, 360)/45)%8]
}

// sunRow: sunrise and sunset in Oslo today, how long the day is, and how
// much it changed since yesterday. Empty on a day the sun doesn't rise or
// set.
func sunRow(now time.Time) Row {
	r := Row{Label: "Sol"}
	rise, set, ok := sun.Times(now, osloLat, osloLon)
	if !ok {
		return r
	}
	r.Value = rise.Format("15:04") + "–" + set.Format("15:04")
	length := set.Sub(rise)
	r.Details = []string{duration(length) + " dagslys"}
	if rise0, set0, ok := sun.Times(now.AddDate(0, 0, -1), osloLat, osloLon); ok {
		diff := int(tz.Round((length - set0.Sub(rise0)).Minutes()))
		switch {
		case diff < 0:
			r.Details = append(r.Details, strconv.Itoa(-diff)+nbsp+"min kortere enn i går")
		case diff > 0:
			r.Details = append(r.Details, strconv.Itoa(diff)+nbsp+"min lengre enn i går")
		}
	}
	return r
}

// duration is "9 t 8 min".
func duration(d time.Duration) string {
	m := int(tz.Round(d.Minutes()))
	return strconv.Itoa(m/60) + nbsp + "t " + strconv.Itoa(m%60) + nbsp + "min"
}
