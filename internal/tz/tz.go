// Package tz holds the one time zone Tellulf cares about, plus a couple of
// rounding/formatting helpers that mimic JavaScript so the port renders the
// same numbers as the TypeScript version did.
package tz

import (
	"math"
	"time"
	_ "time/tzdata" // embed the zone database so scratch/distroless images work
)

// Oslo is used for every wall-clock decision: what "today" is, which hour a
// forecast belongs to, how times are printed.
var Oslo = mustLoad("Europe/Oslo")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Now returns the current time in Oslo.
func Now() time.Time { return time.Now().In(Oslo) }

// StartOfDay returns local midnight for t's date in Oslo.
func StartOfDay(t time.Time) time.Time {
	t = t.In(Oslo)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Oslo)
}

// SameDay reports whether a and b fall on the same Oslo calendar date.
func SameDay(a, b time.Time) bool {
	a, b = a.In(Oslo), b.In(Oslo)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// Round is JavaScript's Math.round: halves round towards +Infinity, so
// Round(-2.5) == -2 (Go's math.Round would give -3).
func Round(x float64) float64 { return math.Floor(x + 0.5) }
