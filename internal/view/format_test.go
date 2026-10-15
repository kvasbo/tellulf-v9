package view

import (
	"math"
	"testing"
)

func TestFixed(t *testing.T) {
	cases := []struct {
		x    float64
		d    int
		want string
	}{
		{1.25, 1, "1.3"},
		{0.125, 2, "0.13"},
		{2.5, 0, "3"},
		{-2.5, 0, "-3"},
		{1.005, 2, "1.00"}, // 1.005 is really 1.00499999…
		{0, 2, "0.00"},
		{math.Copysign(0, -1), 1, "0.0"},
		{-0.001, 2, "0.00"},
		{-0.3, 0, "0"},
		{-1.24, 1, "-1.2"},
		{12345.678, 1, "12345.7"},
		{7, 2, "7.00"},
	}
	for _, c := range cases {
		if got := Fixed(c.x, c.d); got != c.want {
			t.Errorf("Fixed(%v, %d) = %q, want %q", c.x, c.d, got, c.want)
		}
	}
}

func TestNum(t *testing.T) {
	cases := map[float64]string{
		7.3:                  "7.3",
		10:                   "10",
		-9999:                "-9999",
		math.Copysign(0, -1): "0",
		0.5:                  "0.5",
	}
	for x, want := range cases {
		if got := Num(x); got != want {
			t.Errorf("Num(%v) = %q, want %q", x, got, want)
		}
	}
}

func TestWeatherIcon(t *testing.T) {
	cases := map[string]string{
		"clearsky_night":     "/weather-icons-static/clear-night.svg",
		"fair_night":         "/weather-icons-static/partly-cloudy-night.svg",
		"partlycloudy_night": "/weather-icons-static/partly-cloudy-night.svg",
		"clearsky_day":       "/weather-icons-animated/clear-day.svg",
		"rainshowers_night":  "/weather-icons-animated/partly-cloudy-night-rain.svg",
		"no_such_symbol":     "",
	}
	for symbol, want := range cases {
		if got := weatherIcon(symbol); got != want {
			t.Errorf("%s: got %q, want %q", symbol, got, want)
		}
	}
}
