package sky

import (
	"math"
	"strings"
	"testing"
	"time"
)

func at(h, m int) time.Time { return time.Date(2026, 6, 7, h, m, 0, 0, time.Local) }

var (
	sunrise = at(4, 0)
	sunset  = at(22, 0)
)

func TestGetPhase(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want Phase
	}{
		{"midday is day", at(13, 0), Day},
		{"deep night is night", at(1, 0), Night},
		{"just after sunrise is dawn", at(4, 10), Dawn},
		{"just before sunrise is dawn", at(3, 30), Dawn},
		{"around sunset is dusk", at(22, 20), Dusk},
		{"outside the window flips to day", at(5, 0), Day},
		{"outside the window flips to night", at(23, 0), Night},
	}
	for _, c := range cases {
		if got := GetPhase(c.now, sunrise, sunset, DefaultTwilight); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestGetCondition(t *testing.T) {
	cases := map[string]Condition{
		"clearsky_day":         Clear,
		"clearsky_night":       Clear,
		"partlycloudy_day":     Partly,
		"fair_night":           Partly,
		"cloudy":               Cloudy,
		"fog":                  Cloudy,
		"lightrain":            Precip,
		"heavysnowshowers_day": Precip,
		"sleet":                Precip,
		"rainandthunder":       Precip,
		"":                     Cloudy, // missing symbol
	}
	for symbol, want := range cases {
		if got := GetCondition(symbol); got != want {
			t.Errorf("%q: got %s, want %s", symbol, got, want)
		}
	}
}

func TestGetPrecip(t *testing.T) {
	temp := func(f float64) *float64 { return &f }
	cases := []struct {
		symbol string
		temp   *float64
		want   Precipitation
	}{
		{"clearsky_day", temp(10), None},
		{"lightrain", temp(8), Rain},
		{"heavysnow", temp(-3), Snow},
		{"sleet", temp(-2), Snow}, // sleet falls back to temperature
		{"sleet", temp(6), Rain},
		{"sleet", nil, Rain}, // unknown temperature counts as 5°C
	}
	for _, c := range cases {
		if got := GetPrecip(c.symbol, c.temp); got != c.want {
			t.Errorf("%s: got %s, want %s", c.symbol, got, c.want)
		}
	}
}

func TestGetArc(t *testing.T) {
	near := func(got, want, tol float64) bool { return math.Abs(got-want) <= tol }
	if a := GetArc(sunrise, sunrise, sunset); !near(a, 0, 1e-5) {
		t.Errorf("sunrise: got %v, want 0", a)
	}
	if a := GetArc(sunset, sunrise, sunset); !near(a, 1, 1e-5) {
		t.Errorf("sunset: got %v, want 1", a)
	}
	if a := GetArc(at(13, 0), sunrise, sunset); !near(a, 0.5, 0.05) {
		t.Errorf("13:00: got %v, want ~0.5", a)
	}
	if a := GetArc(at(1, 0), sunrise, sunset); a < 0 || a > 1 {
		t.Errorf("night progress out of range: %v", a)
	}
}

func TestBuildStateCoversEveryCombination(t *testing.T) {
	for _, phase := range []Phase{Night, Dawn, Day, Dusk} {
		for _, cond := range []Condition{Clear, Partly, Cloudy, Precip} {
			s := BuildState(phase, cond, None, 0.5)
			for _, c := range []RGB{s.Colors.C1, s.Colors.C2, s.Colors.C3} {
				for _, ch := range c {
					if ch < 0 || ch > 1 {
						t.Errorf("%s/%s: channel %v out of range", phase, cond, ch)
					}
				}
			}
			if !strings.HasPrefix(s.CSS.Fg, "#") && !strings.Contains(s.CSS.Fg, "rgba") {
				t.Errorf("%s/%s: odd fg %q", phase, cond, s.CSS.Fg)
			}
			if !strings.Contains(s.CSS.GlassBg, "rgba") {
				t.Errorf("%s/%s: odd glass bg %q", phase, cond, s.CSS.GlassBg)
			}
		}
	}
}

func TestPrecipFlattensGradient(t *testing.T) {
	spread := func(s State) float64 { return math.Abs(s.Colors.C1[2] - s.Colors.C1[0]) }
	clear := BuildState(Day, Clear, None, 0.5)
	precip := BuildState(Day, Precip, Rain, 0.5)
	if spread(precip) >= spread(clear) {
		t.Errorf("precip spread %v should be below clear spread %v", spread(precip), spread(clear))
	}
}
