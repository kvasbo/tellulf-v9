// Package sun computes sunrise and sunset using NOAA's solar calculator
// equations (https://gml.noaa.gov/grad/solcalc/calcdetails.html). That is
// accurate to about a minute at mid latitudes, which is all a clock-face
// display needs, and fits in a page instead of NREL's full SPA tables.
package sun

import (
	"math"
	"time"
)

// zenith for sunrise/sunset: 90° plus atmospheric refraction (0.5667°) plus
// the sun's apparent radius (0.26667°).
const zenith = 90.8333

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

func julianDay(t time.Time) float64 {
	return float64(t.UnixNano())/float64(24*time.Hour) + 2440587.5
}

// solar returns the sun's declination (degrees) and the equation of time
// (minutes) at the given Julian day.
func solar(jd float64) (decl, eqTime float64) {
	t := (jd - 2451545.0) / 36525 // Julian centuries since J2000.0

	l0 := math.Mod(280.46646+t*(36000.76983+t*0.0003032), 360) // mean longitude
	m := rad(357.52911 + t*(35999.05029-0.0001537*t))          // mean anomaly
	e := 0.016708634 - t*(0.000042037+0.0000001267*t)          // orbit eccentricity

	center := math.Sin(m)*(1.914602-t*(0.004817+0.000014*t)) +
		math.Sin(2*m)*(0.019993-0.000101*t) +
		math.Sin(3*m)*0.000289
	omega := rad(125.04 - 1934.136*t)
	apparentLong := rad(l0 + center - 0.00569 - 0.00478*math.Sin(omega))

	seconds := 21.448 - t*(46.815+t*(0.00059-t*0.001813))
	obliquity := rad(23 + (26+seconds/60)/60 + 0.00256*math.Cos(omega))

	decl = deg(math.Asin(math.Sin(obliquity) * math.Sin(apparentLong)))

	y := math.Pow(math.Tan(obliquity/2), 2)
	l0r := rad(l0)
	eqTime = 4 * deg(y*math.Sin(2*l0r)-
		2*e*math.Sin(m)+
		4*e*y*math.Sin(m)*math.Cos(2*l0r)-
		0.5*y*y*math.Sin(4*l0r)-
		1.25*e*e*math.Sin(2*m))
	return decl, eqTime
}

// event finds sunrise (dir=-1) or sunset (dir=+1) on the UTC date starting at
// midnight. Two refinement passes recompute the sun's position at the
// estimated event time instead of at noon.
func event(midnight time.Time, lat, lon float64, dir float64) (time.Time, bool) {
	minutes := 720 - 4*lon // first guess: solar noon
	for range 3 {
		jd := julianDay(midnight.Add(time.Duration(minutes * float64(time.Minute))))
		decl, eqTime := solar(jd)
		cosHA := math.Cos(rad(zenith))/(math.Cos(rad(lat))*math.Cos(rad(decl))) -
			math.Tan(rad(lat))*math.Tan(rad(decl))
		if cosHA < -1 || cosHA > 1 {
			return time.Time{}, false // midnight sun or polar night
		}
		ha := deg(math.Acos(cosHA))
		minutes = 720 - 4*(lon-dir*ha) - eqTime
	}
	return midnight.Add(time.Duration(math.Round(minutes * float64(time.Minute)))), true
}

// Times returns sunrise and sunset for the calendar date of day (in day's own
// location). ok is false when the sun doesn't rise or set that day.
func Times(day time.Time, lat, lon float64) (rise, set time.Time, ok bool) {
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	rise, okRise := event(midnight, lat, lon, -1)
	set, okSet := event(midnight, lat, lon, +1)
	if !okRise || !okSet {
		return time.Time{}, time.Time{}, false
	}
	return rise.In(day.Location()), set.In(day.Location()), true
}
