package view

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kvasbo/tellulf-v9/internal/tz"
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

// Norwegian typography: decimal comma, a real minus sign, and a narrow
// no-break space between a number and its unit.
const (
	minus = "−" // U+2212
	nbsp  = " "
)

// nb turns a Go-formatted number ("-0.4") into Norwegian ("−0,4").
func nb(s string) string {
	s = strings.ReplaceAll(s, ".", ",")
	return strings.Replace(s, "-", minus, 1)
}

// Deg formats a temperature in whole degrees: "11°", "−3°", never "−0°".
func Deg(v float64) string { return nb(Num(tz.Round(v))) + "°" }

// DegRange formats a min–max span: "5–12°".
func DegRange(lo, hi float64) string { return nb(Num(tz.Round(lo))) + "–" + Deg(hi) }

// KW formats watts as kilowatts with one decimal: "0,0 kW", "−0,4 kW".
func KW(watts float64) string { return nb(Fixed(watts/1000, 1)) + nbsp + "kW" }

// KWh formats energy with one decimal below 10 kWh and whole kWh from
// there: "3,1 kWh", "48 kWh".
func KWh(v float64) string {
	if math.Abs(v) >= 9.95 {
		return nb(Num(tz.Round(v))) + nbsp + "kWh"
	}
	return nb(Fixed(v, 1)) + nbsp + "kWh"
}

// Kr formats a price with two decimals: "0,50 kr".
func Kr(v float64) string { return nb(Fixed(v, 2)) + nbsp + "kr" }

// Pct formats a whole-number percentage: "27 %".
func Pct(v float64) string { return nb(Num(tz.Round(v))) + nbsp + "%" }

// Capitalize upper-cases the first letter: "tirsdag" → "Tirsdag".
func Capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// weekdays is indexed by time.Weekday.
var weekdays = [...]string{"søndag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag"}

// months is indexed by time.Month - 1.
var months = [...]string{"januar", "februar", "mars", "april", "mai", "juni", "juli",
	"august", "september", "oktober", "november", "desember"}

// longDate is "Tirsdag 27. oktober".
func longDate(t time.Time) string {
	t = t.In(tz.Oslo)
	return Capitalize(weekdays[t.Weekday()]) + " " + strconv.Itoa(t.Day()) + ". " + months[t.Month()-1]
}

// daysBetween counts calendar days in Oslo from a to b; DST days count as one.
func daysBetween(a, b time.Time) int {
	return int(tz.Round(tz.StartOfDay(b).Sub(tz.StartOfDay(a)).Hours() / 24))
}

// dayRef names t's date as seen from the day from: the weekday within six
// days, and the weekday with the date from a week on, so a name never
// means two days: "fredag", "mandag 9. november".
func dayRef(t, from time.Time) string {
	t = t.In(tz.Oslo)
	ref := weekdays[t.Weekday()]
	if daysBetween(from, t) >= 7 {
		ref += " " + strconv.Itoa(t.Day()) + ". " + months[t.Month()-1]
	}
	return ref
}

// MM formats precipitation with one decimal: "3,2 mm".
func MM(v float64) string { return nb(Fixed(v, 1)) + nbsp + "mm" }

// MS formats wind in whole metres per second: "4 m/s".
func MS(v float64) string { return nb(Num(tz.Round(v))) + nbsp + "m/s" }
