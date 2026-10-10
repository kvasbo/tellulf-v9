// Package view turns raw data into exactly what the templates print. All
// formatting and arithmetic happens here, so the templates stay dumb.
package view

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/sky"
	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/sun"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

const (
	osloLat = 59.9508
	osloLon = 10.6847
)

// --- Current weather -----------------------------------------------------------

type CurrentWeather struct {
	Temperature     string
	Sunrise, Sunset string
	Pressure        string
	Humidity        string
	ShowTempTime    bool // the outdoor sensor has gone quiet
	TempTimeStr     string
}

func BuildCurrentWeather(r smarthouse.Readings, now time.Time) CurrentWeather {
	cw := CurrentWeather{
		Temperature: "–",
		Pressure:    orDash(r.Pressure, func(v float64) string { return Num(tz.Round(v)) }),
		Humidity:    orDash(r.HumOut, func(v float64) string { return Num(tz.Round(v)) + "%" }),
	}
	if r.TempOut != nil {
		cw.Temperature = Num(*r.TempOut) + "°"
	}
	if rise, set, ok := sun.Times(now.In(tz.Oslo), osloLat, osloLon); ok {
		cw.Sunrise = rise.Format("15:04")
		cw.Sunset = set.Format("15:04")
	}
	if !r.LastTempTime.IsZero() {
		cw.ShowTempTime = now.Sub(r.LastTempTime) > 20*time.Minute
		cw.TempTimeStr = r.LastTempTime.In(tz.Oslo).Format("15:04:05")
	}
	return cw
}

// orDash formats a sensor value, or "–" when the sensor hasn't reported.
func orDash(v *float64, format func(float64) string) string {
	if v == nil {
		return "–"
	}
	return format(*v)
}

// --- Hourly forecast -------------------------------------------------------------

// Only show the wind indicator from this speed (m/s).
const windDisplayThreshold = 5

type HourlyForecast struct {
	Forecasts       []ForecastHour
	Min, Max        string
	DisplayZeroLine string
	ZeroLineBottom  string
}

type ForecastHour struct {
	Hour          int
	Raining       bool
	RainHeight    string
	WaveSrc       string
	Bottom        string // vertical position from the temperature
	IconSrc       string
	Temperature   string
	ShowProb      bool
	Probability   string
	Amount        string
	ShowWind      bool
	WindSpeed     string
	ShowGust      bool
	WindGust      string
	WindDirection string
}

func weatherIcon(symbol string) string {
	name, ok := weatherIconMapping[symbol]
	if !ok {
		return ""
	}
	// The night icons without precipitation use the still versions.
	folder := "animated"
	if slices.Contains([]string{"clear-night", "partly-cloudy-night"}, name) {
		folder = "static"
	}
	return "/weather-icons-" + folder + "/" + name + ".svg"
}

func rainHeight(mm float64) float64 { return math.Min(100, mm*17) }

// minMaxTemps picks the temperature axis: at least 5° of headroom rounded
// out to multiples of 5, always including 0, and spanning at least 20°.
func minMaxTemps(hours []weather.Hourly) (lo, hi float64) {
	lo, hi = 100, -100
	for _, h := range hours {
		t := deref(h.AirTemperature)
		lo, hi = math.Min(lo, t), math.Max(hi, t)
	}
	hi = math.Max(math.Ceil((hi+5)/5)*5, 0)
	lo = math.Min(math.Floor((lo-5)/5)*5, 0)
	if hi-lo < 20 {
		hi = lo + 20
	}
	return lo, hi
}

func deref(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func BuildHourlyForecast(all []weather.Hourly) HourlyForecast {
	// Skip the current hour, show the next 18.
	hours := all[min(1, len(all)):min(19, len(all))]
	if len(hours) == 0 {
		return HourlyForecast{DisplayZeroLine: "none"}
	}
	lo, hi := minMaxTemps(hours)
	toRange := func(v float64) string { return Num((v - lo) / (hi - lo) * 100) }

	out := HourlyForecast{
		Min:             Num(lo),
		Max:             Num(hi),
		DisplayZeroLine: "none",
		ZeroLineBottom:  toRange(0),
	}
	if hi > 0 && lo < 0 {
		out.DisplayZeroLine = "block"
	}
	for _, h := range hours {
		temp := deref(h.AirTemperature)
		speed := tz.Round(deref(h.WindSpeed))
		gust := tz.Round(deref(h.WindSpeedOfGust))
		out.Forecasts = append(out.Forecasts, ForecastHour{
			Hour:       h.Hour,
			Raining:    h.PrecipitationAmount > 0,
			RainHeight: Num(rainHeight(h.PrecipitationAmount)),
			// Deterministic wave pick (1-3) so re-renders stay identical.
			WaveSrc:       "/wave" + Num(float64(h.Hour%3+1)) + ".svg",
			Bottom:        toRange(temp),
			IconSrc:       weatherIcon(h.Symbol),
			Temperature:   Num(temp),
			ShowProb:      h.ProbabilityOfPrecipitation != 0,
			Probability:   Num(h.ProbabilityOfPrecipitation),
			Amount:        Num(h.PrecipitationAmount),
			ShowWind:      deref(h.WindSpeed) >= windDisplayThreshold,
			WindSpeed:     Num(speed),
			ShowGust:      gust > speed+3,
			WindGust:      Num(gust),
			WindDirection: Num(deref(h.WindFromDirection)),
		})
	}
	return out
}

// --- Living sky ------------------------------------------------------------------

// Sky feeds the WebGL background and the adaptive text/glass colours.
type Sky struct {
	// RootCSS sets the theme variables, e.g. ":root{--fg:#1a1d22;...}". It is
	// built only from the constant palettes in package sky, so it is safe to
	// write into a <style> element unescaped.
	RootCSS           string
	C1, C2, C3        string
	Arc, Cloud, Night string
}

var cloudCoverage = map[sky.Condition]float64{sky.Clear: 0.05, sky.Partly: 0.4, sky.Cloudy: 0.85, sky.Precip: 1.0}
var nightFactor = map[sky.Phase]float64{sky.Night: 1, sky.Dawn: 0.5, sky.Dusk: 0.5, sky.Day: 0}

func rootCSS(c sky.CSS) string {
	return ":root{--fg:" + c.Fg + ";--fg-muted:" + c.FgMuted + ";--fg-faint:" + c.FgFaint +
		";--line:" + c.Line + ";--glass-bg:" + c.GlassBg + ";--glass-border:" + c.GlassBorder +
		";--glass-shadow:" + c.GlassShadow + ";--icon-filter:" + c.IconFilter + ";}"
}

func rgb(c sky.RGB) string {
	return Fixed(c[0], 3) + "," + Fixed(c[1], 3) + "," + Fixed(c[2], 3)
}

func BuildSky(hours []weather.Hourly, now time.Time) Sky {
	rise, set, ok := sun.Times(now.In(tz.Oslo), osloLat, osloLon)
	if !ok {
		rise, set = now, now
	}
	var symbol string
	var temp *float64
	if len(hours) > 0 {
		symbol, temp = hours[0].Symbol, hours[0].AirTemperature
	}

	phase := sky.GetPhase(now, rise, set, sky.DefaultTwilight)
	condition := sky.GetCondition(symbol)
	arc := sky.GetArc(now, rise, set)
	s := sky.BuildState(phase, condition, sky.GetPrecip(symbol, temp), arc)

	return Sky{
		RootCSS: rootCSS(s.CSS),
		C1:      rgb(s.Colors.C1),
		C2:      rgb(s.Colors.C2),
		C3:      rgb(s.Colors.C3),
		Arc:     Fixed(arc, 4),
		Cloud:   Fixed(cloudCoverage[condition], 2),
		Night:   Fixed(nightFactor[phase], 2),
	}
}

// --- Power -----------------------------------------------------------------------

type Power struct {
	Header        string
	HasData       bool
	Usage         string
	Cost          string
	Price         string
	MonthlyStatus string
	MonthlyCost   string
	PriceStatus   string
	PriceColor    string
	BgColor       string
	PowerDisplay  string
	PowerWidth    string
	MaxWidth      string
	AvgWidth      string
	MinWidth      string
	MinBgColor    string
}

const (
	green  = "#3ef0a4"
	orange = "#f0a43e"
	blue   = "#3ea4f0"
)

func BuildPower(d tibber.PowerData, place tibber.Place, now time.Time) Power {
	now = now.In(tz.Oslo)
	norgespris := d.Cap != 0
	underCap := norgespris && d.MonthlyConsumption < d.Cap

	p := Power{
		Header:        map[tibber.Place]string{tibber.Home: "Hjemme", tibber.Cabin: "Hytta"}[place],
		HasData:       !d.Timestamp.IsZero(),
		Usage:         Fixed(d.AccumulatedConsumption, 2) + " kWh",
		Cost:          Fixed(d.AccumulatedCost, 2) + " kr",
		MonthlyStatus: Fixed(d.MonthlyConsumption, 1) + " kWh",
		MonthlyCost:   Fixed(d.MonthlyCost, 0) + " kr",
		PriceStatus:   "Spotpris",
		PriceColor:    orange,
		BgColor:       blue,
		PowerDisplay:  Fixed(d.CurrentPower/1000, 1),
		MinBgColor:    blue + "33",
	}
	price := d.EffectivePrice
	if price == 0 {
		price = d.CurrentPrice
	}
	p.Price = Fixed(price, 2) + " kr"

	// 30 November is "Megajouleens dag": show energy in MJ.
	if now.Month() == time.November && now.Day() == 30 {
		p.Usage = Num(tz.Round(d.AccumulatedConsumption*3.6)) + " MJ"
	}

	if norgespris {
		// How far ahead or behind a straight-line use of the cap we are.
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
		monthEnd := monthStart.AddDate(0, 1, 0)
		fraction := float64(now.Sub(monthStart)) / float64(monthEnd.Sub(monthStart))
		expected := math.Max(fraction*d.Cap, 1)
		diff := (d.MonthlyConsumption - expected) / expected * 100
		sign := ""
		if diff > 0 {
			sign = "+"
		}
		p.MonthlyStatus += " (" + sign + Fixed(diff, 0) + "%)"
	}
	if underCap {
		p.PriceStatus, p.PriceColor = "Norgespris", green
	}
	if d.CurrentPower < 0 {
		p.BgColor = green
	}

	// Bar widths are whole percent of 30 kW, so per-second watt jitter doesn't
	// produce new HTML every tick (which would defeat the SSE dedup).
	const maxPower = 30000.0 / 100
	minP := d.MinPower
	if d.MaxPowerProduction > 0 {
		minP = math.Min(d.MinPower, -d.MaxPowerProduction)
	}
	if minP < 0 {
		p.MinBgColor = green
	}
	p.PowerWidth = Num(tz.Round(math.Abs(d.CurrentPower / maxPower)))
	p.MaxWidth = Num(tz.Round(d.MaxPower / maxPower))
	p.AvgWidth = Num(tz.Round(d.AveragePower / maxPower))
	p.MinWidth = Num(tz.Round(math.Abs(minP / maxPower)))
	return p
}

// --- Entur -----------------------------------------------------------------------

type Train struct {
	Time        string
	Destination string
	Dot         string // "red" when delayed, "grey" without realtime data
}

func BuildTrains(trains []entur.Train) []Train {
	var out []Train
	for _, t := range trains[:min(4, len(trains))] {
		dot := ""
		switch {
		case t.Delayed:
			dot = "red"
		case !t.Realtime:
			dot = "grey"
		}
		out = append(out, Train{Time: t.Time.In(tz.Oslo).Format("15:04"), Destination: t.Destination, Dot: dot})
	}
	return out
}

// --- Calendar --------------------------------------------------------------------

type Calendar struct {
	Days        []Day
	BhgDaysLeft *int
}

type Day struct {
	Weekday    string
	KidsStatus calendar.KidsStatus
	Forecast   *DayForecast
	Dinners    []calendar.EnrichedEvent
	Birthdays  []calendar.EnrichedEvent
	Events     []Event
}

type DayForecast struct {
	ShowLightRain, ShowHeavyRain bool
	LightRain, HeavyRain         string
	MinTemp, MaxTemp             string
}

type Event struct {
	calendar.EnrichedEvent
	Hytta *HyttaWeather
}

type HyttaWeather struct{ Temperature, RainProbability string }

// BuildCalendar lays out the coming days, starting today.
func BuildCalendar(cal calendar.Snapshot, daily, dailyHytta map[string]weather.Daily, now time.Time, days int) Calendar {
	out := Calendar{BhgDaysLeft: cal.BhgDaysLeft}
	now = now.In(tz.Oslo)
	for i := range days {
		date := now.AddDate(0, 0, i)
		iso := date.Format(time.DateOnly)

		d := Day{
			Weekday:    niceDate(date),
			KidsStatus: cal.KidsStatusForDate(date),
			Dinners:    cal.DinnersForDate(date),
			Birthdays:  cal.BirthdaysForDate(date),
		}
		switch i {
		case 0:
			d.Weekday = "i dag"
		case 1:
			d.Weekday = "i morgen"
		}
		if f, ok := daily[iso]; ok {
			d.Forecast = &DayForecast{
				ShowLightRain: f.LightRainProbability > 10,
				ShowHeavyRain: f.HeavyRainProbability > 10,
				LightRain:     Num(f.LightRainProbability),
				HeavyRain:     Num(f.HeavyRainProbability),
				MinTemp:       Num(tz.Round(f.MinTemp)),
				MaxTemp:       Num(tz.Round(f.MaxTemp)),
			}
		}
		for _, e := range cal.EventsForDate(date) {
			ev := Event{EnrichedEvent: e}
			// Full-day "Hytta" events get the cabin's forecast.
			if f, ok := dailyHytta[iso]; ok && e.FullDay && strings.Contains(strings.ToLower(e.Title), "hytta") {
				ev.Hytta = &HyttaWeather{Temperature: Num(tz.Round(f.MaxTemp)), RainProbability: Num(f.LightRainProbability)}
			}
			d.Events = append(d.Events, ev)
		}
		out.Days = append(out.Days, d)
	}
	return out
}
