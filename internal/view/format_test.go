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

func TestDeg(t *testing.T) {
	cases := map[float64]string{11.4: "11°", 11.5: "12°", -3.2: "−3°", -0.4: "0°", 0: "0°"}
	for in, want := range cases {
		if got := Deg(in); got != want {
			t.Errorf("Deg(%v) = %q, want %q", in, got, want)
		}
	}
	if got := DegRange(4.6, 12.2); got != "5–12°" {
		t.Errorf("DegRange = %q", got)
	}
	if got := DegRange(-3.4, 2); got != "−3–2°" {
		t.Errorf("DegRange = %q", got)
	}
}

func TestUnits(t *testing.T) {
	cases := []struct{ got, want string }{
		{KW(0), "0,0 kW"},
		{KW(-400), "−0,4 kW"},
		{KW(-40), "0,0 kW"},
		{KW(12345), "12,3 kW"},
		{KWh(14.25), "14,3 kWh"},
		{KWh(-0.04), "0,0 kWh"},
		{Kr(0.5), "0,50 kr"},
		{Pct(27.4), "27 %"},
		{Capitalize("tirsdag"), "Tirsdag"},
		{Capitalize(""), ""},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
