package view

import (
	"math"
	"strconv"
	"time"
)

// Num formats a number with as many decimals as it needs: "7.3", "10".
func Num(x float64) string {
	return strconv.FormatFloat(x+0, 'f', -1, 64) // +0 turns -0 into 0
}

// Fixed formats x with exactly digits decimals, rounding halves away from
// zero ("1.3" for 1.25), and never as "-0.0".
func Fixed(x float64, digits int) string {
	p := math.Pow10(digits)
	return strconv.FormatFloat(math.Round(x*p)/p+0, 'f', digits, 64)
}

var weekdays = [...]string{"søndag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag"}

// niceDate formats a date like "mandag 10."
func niceDate(t time.Time) string {
	return weekdays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + "."
}
