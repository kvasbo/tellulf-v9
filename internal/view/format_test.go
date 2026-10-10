package view

import (
	"math"
	"testing"
)

func TestFixedMatchesJavaScript(t *testing.T) {
	// Expected values are what (x).toFixed(d) returns in JavaScript.
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
		{-0.001, 2, "-0.00"},
		{-0.3, 0, "-0"},
		{12345.678, 1, "12345.7"},
		{0.05, 1, "0.1"}, // 0.05 is really 0.05000000000000000277
		{7, 2, "7.00"},
	}
	for _, c := range cases {
		if got := Fixed(c.x, c.d); got != c.want {
			t.Errorf("Fixed(%v, %d) = %q, want %q", c.x, c.d, got, c.want)
		}
	}
}

func TestNumMatchesJavaScript(t *testing.T) {
	a, b := 0.1, 0.2 // variables, so Go doesn't fold 0.1+0.2 into an exact 0.3
	cases := map[float64]string{
		7.3:                  "7.3",
		10:                   "10",
		-9999:                "-9999",
		math.Copysign(0, -1): "0",
		100.0 / 3:            "33.333333333333336",
		a + b:                "0.30000000000000004",
		1e-7:                 "1e-7",
		1.5e21:               "1.5e+21",
	}
	for x, want := range cases {
		if got := Num(x); got != want {
			t.Errorf("Num(%v) = %q, want %q", x, got, want)
		}
	}
}
