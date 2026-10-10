// Package sky holds the pure logic for the living sky background.
// The server owns all weather/time reasoning; the client only renders.
// This package turns (time, sun times, weather symbol) into a State:
//   - numeric uniforms for the WebGL shader
//   - CSS variable strings for adaptive text/glass readability
package sky

import (
	"math"
	"regexp"
	"strings"
	"time"
)

type Phase string

const (
	Night Phase = "night"
	Dawn  Phase = "dawn"
	Day   Phase = "day"
	Dusk  Phase = "dusk"
)

type Condition string

const (
	Clear  Condition = "clear"
	Partly Condition = "partly"
	Cloudy Condition = "cloudy"
	Precip Condition = "precip"
)

type Precipitation string

const (
	None Precipitation = "none"
	Rain Precipitation = "rain"
	Snow Precipitation = "snow"
)

// RGB is a colour with each channel in 0..1.
type RGB [3]float64

// Gradient holds the three stops (top, middle, horizon) fed to the shader.
type Gradient struct{ C1, C2, C3 RGB }

// CSS holds the adaptive CSS variables for text and glass.
type CSS struct {
	Fg, FgMuted, FgFaint, Line        string
	GlassBg, GlassBorder, GlassShadow string
	IconFilter                        string // filter() to recolour monochrome icons
}

type State struct {
	Phase     Phase
	Condition Condition
	Precip    Precipitation
	Arc       float64 // 0..1 position of the luminary along its arc
	Colors    Gradient
	CSS       CSS
}

const dayLength = 24 * time.Hour

func clamp01(n float64) float64 { return math.Min(1, math.Max(0, n)) }

// DefaultTwilight is the window on each side of a sun event that counts as
// dawn/dusk.
const DefaultTwilight = 45 * time.Minute

// GetPhase returns the time-of-day phase. Dawn/dusk are windows of twilight
// on each side of the sun event; outside them it's day (sun up) or night.
func GetPhase(now, sunrise, sunset time.Time, twilight time.Duration) Phase {
	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return -d
		}
		return d
	}
	switch {
	case abs(now.Sub(sunrise)) <= twilight:
		return Dawn
	case abs(now.Sub(sunset)) <= twilight:
		return Dusk
	case now.After(sunrise) && now.Before(sunset):
		return Day
	default:
		return Night
	}
}

var precipPattern = regexp.MustCompile(`rain|snow|sleet|thunder`)

// GetCondition buckets a MET Norway symbol code into a coarse condition.
func GetCondition(symbol string) Condition {
	if symbol == "" {
		return Cloudy
	}
	s := strings.ToLower(symbol)
	switch {
	case precipPattern.MatchString(s):
		return Precip
	case strings.Contains(s, "clearsky"):
		return Clear
	case strings.Contains(s, "partlycloudy"), strings.Contains(s, "fair"):
		return Partly
	default:
		return Cloudy // cloudy, fog, unknown
	}
}

// GetPrecip decides rain vs snow, using the symbol and then the temperature.
// A nil temperature counts as 5°C.
func GetPrecip(symbol string, airTemperature *float64) Precipitation {
	if GetCondition(symbol) != Precip {
		return None
	}
	s := strings.ToLower(symbol)
	if strings.Contains(s, "snow") {
		return Snow
	}
	if strings.Contains(s, "rain") {
		return Rain
	}
	// sleet / ambiguous: fall back to temperature
	temp := 5.0
	if airTemperature != nil {
		temp = *airTemperature
	}
	if temp <= 1 {
		return Snow
	}
	return Rain
}

// GetArc returns 0..1 progress of the luminary along its arc. By day it tracks
// the sun from sunrise (0) to sunset (1); at night it tracks the moon across
// the dark hours.
func GetArc(now, sunrise, sunset time.Time) float64 {
	dayMs := sunset.Sub(sunrise)
	if !now.Before(sunrise) && !now.After(sunset) {
		return clamp01(float64(now.Sub(sunrise)) / float64(dayMs))
	}
	nightMs := dayLength - dayMs
	// After sunset: progress from this sunset toward the next sunrise.
	// Before sunrise: progress from the previous sunset (set - 1 day).
	nightStart := sunset
	if !now.After(sunset) {
		nightStart = sunset.Add(-dayLength)
	}
	return clamp01(float64(now.Sub(nightStart)) / float64(nightMs))
}

// --- Palettes ---------------------------------------------------------------

var phaseGradient = map[Phase]Gradient{
	Night: {C1: RGB{0.03, 0.04, 0.11}, C2: RGB{0.06, 0.08, 0.17}, C3: RGB{0.11, 0.13, 0.24}},
	Dawn:  {C1: RGB{0.18, 0.22, 0.42}, C2: RGB{0.52, 0.43, 0.54}, C3: RGB{0.98, 0.71, 0.5}},
	Day:   {C1: RGB{0.26, 0.54, 0.85}, C2: RGB{0.5, 0.72, 0.93}, C3: RGB{0.83, 0.92, 0.98}},
	Dusk:  {C1: RGB{0.14, 0.16, 0.36}, C2: RGB{0.5, 0.35, 0.46}, C3: RGB{0.96, 0.55, 0.4}},
}

// How much each condition flattens (desaturates) and darkens the gradient.
var conditionMod = map[Condition]struct{ desat, darken float64 }{
	Clear:  {0, 0},
	Partly: {0.2, 0.02},
	Cloudy: {0.5, 0.06},
	Precip: {0.6, 0.12},
}

func luminance(c RGB) float64 { return 0.299*c[0] + 0.587*c[1] + 0.114*c[2] }

func applyCondition(c RGB, desat, darken float64) RGB {
	l := luminance(c)
	k := 1 - darken
	var out RGB
	for i := range c {
		out[i] = (c[i] + (l-c[i])*desat) * k
	}
	return out
}

// Two readability schemes. Glass background supplies the contrast for the
// text, so each scheme is internally consistent regardless of the raw sky.
var lightUI = CSS{
	Fg:          "#1a1d22",
	FgMuted:     "#4a4f57",
	FgFaint:     "rgba(20, 22, 28, 0.42)",
	Line:        "rgba(20, 30, 45, 0.22)",
	GlassBg:     "rgba(255, 255, 255, 0.16)",
	GlassBorder: "rgba(255, 255, 255, 0.38)",
	GlassShadow: "rgba(30, 50, 80, 0.18)",
	// kids.svg ships as #999 grey; darken it to sit with the dark muted text.
	IconFilter: "brightness(0.45)",
}

var darkUI = CSS{
	Fg:          "#eef2f8",
	FgMuted:     "#aab4c4",
	FgFaint:     "rgba(230, 238, 250, 0.5)",
	Line:        "rgba(200, 215, 240, 0.22)",
	GlassBg:     "rgba(18, 26, 44, 0.32)",
	GlassBorder: "rgba(150, 170, 210, 0.22)",
	GlassShadow: "rgba(0, 0, 10, 0.35)",
	// Lighten the grey icon to match the light muted text at night.
	IconFilter: "brightness(1.25)",
}

// Dawn/day use dark text on light glass; dusk/night use light text on dark glass.
var phaseScheme = map[Phase]CSS{Day: lightUI, Dawn: lightUI, Dusk: darkUI, Night: darkUI}

func BuildState(phase Phase, condition Condition, precip Precipitation, arc float64) State {
	base := phaseGradient[phase]
	mod := conditionMod[condition]
	return State{
		Phase:     phase,
		Condition: condition,
		Precip:    precip,
		Arc:       arc,
		Colors: Gradient{
			C1: applyCondition(base.C1, mod.desat, mod.darken),
			C2: applyCondition(base.C2, mod.desat, mod.darken),
			C3: applyCondition(base.C3, mod.desat, mod.darken),
		},
		CSS: phaseScheme[phase],
	}
}
