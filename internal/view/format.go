package view

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// Num formats a number the way JavaScript's String(x) does, so templates
// print "7.3", "10" and "-0.5" just like the TypeScript version.
func Num(x float64) string {
	if x == 0 {
		return "0" // also turns -0 into "0", as JS does
	}
	if abs := math.Abs(x); abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(x, 'e', -1, 64) // "1e-07"
		mantissa, exp, _ := strings.Cut(s, "e")
		sign, digits := exp[:1], strings.TrimLeft(exp[1:], "0")
		return mantissa + "e" + sign + digits
	}
	return strconv.FormatFloat(x, 'f', -1, 64)
}

// Fixed is JavaScript's x.toFixed(digits). Unlike strconv, which rounds
// exact halves to even, toFixed rounds them away from zero: (1.25).toFixed(1)
// is "1.3". Halves are common here (watts/1000, kWh*0.5), so the arithmetic
// is done exactly with big.Float.
func Fixed(x float64, digits int) string {
	v := new(big.Float).SetPrec(256).SetFloat64(math.Abs(x))
	v.Mul(v, new(big.Float).SetPrec(256).SetFloat64(math.Pow10(digits)))
	v.Add(v, big.NewFloat(0.5))
	n, _ := v.Int(nil) // truncates, i.e. floors the positive value
	s := n.String()
	if digits > 0 {
		if len(s) <= digits {
			s = strings.Repeat("0", digits-len(s)+1) + s
		}
		s = s[:len(s)-digits] + "." + s[len(s)-digits:]
	}
	if x < 0 { // JS keeps the sign even when the result rounds to zero
		s = "-" + s
	}
	return s
}

var weekdays = [...]string{"søndag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag"}

// niceDate formats a date like "mandag 10." (Luxon's "cccc d." in nb).
func niceDate(t time.Time) string {
	return weekdays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + "."
}
