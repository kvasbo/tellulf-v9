// Package view turns raw data into exactly what the templates print. All
// formatting and arithmetic happens here, so the templates stay dumb.
package view

import (
	"math"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/buildinfo"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

const (
	osloLat = 59.9508
	osloLon = 10.6847
)

// --- Corner stamps ---------------------------------------------------------------

// Stamps are the build and server start times shown small in the corner, so
// it's easy to see which version the screen runs and when it last restarted.
type Stamps struct {
	Built, Started string
}

func stamp(t time.Time) string { return t.In(tz.Oslo).Format("02.01 15:04") }

func BuildStamps(started time.Time) Stamps {
	built := "dev"
	if t, ok := buildinfo.Built(); ok {
		built = stamp(t)
	}
	return Stamps{Built: built, Started: stamp(started)}
}

// --- Power -----------------------------------------------------------------------

// Power is one bottom-row cell: the power drawn right now, and under it the
// price per kWh and how the month is pacing against the Norgespris quota.
type Power struct {
	Label   string // "Strøm hjemme", "Hytta"
	HasData bool   // false until the live feed has delivered
	Power   string // "0,0 kW"
	Detail  string // "0,50 kr · −67 %"; just the price without a cap
}

var placeLabels = map[tibber.Place]string{tibber.Home: "Strøm hjemme", tibber.Cabin: "Hytta"}

// BuildPower's percentage compares the month's consumption with a
// straight-line use of the quota (the share of the month gone times the
// cap): "+20 %" means on course to pass the cap and pay spot price before
// the month ends, "−67 %" well within it.
func BuildPower(d tibber.PowerData, place tibber.Place, now time.Time) Power {
	p := Power{Label: placeLabels[place], HasData: !d.Timestamp.IsZero()}
	if !p.HasData {
		return p
	}
	// Tenths of a kW, so per-second watt jitter doesn't produce new HTML
	// every tick (which would defeat the SSE dedup).
	p.Power = KW(d.CurrentPower)
	price := d.EffectivePrice
	if price == 0 {
		price = d.CurrentPrice
	}
	p.Detail = Kr(price)
	if d.Cap > 0 {
		p.Detail += " · " + paceVsQuota(d.MonthlyConsumption, d.Cap, now)
	}
	return p
}

// paceVsQuota is how far ahead (+) or behind (−) a straight-line use of cap
// over the month the consumption is, in whole percent: "+20 %", "−67 %", "0 %".
func paceVsQuota(consumption, cap float64, now time.Time) string {
	now = now.In(tz.Oslo)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
	monthEnd := monthStart.AddDate(0, 1, 0)
	gone := float64(now.Sub(monthStart)) / float64(monthEnd.Sub(monthStart))
	expected := math.Max(gone*cap, 1) // at midnight on the 1st nothing is expected yet
	diff := tz.Round((consumption - expected) / expected * 100)
	if diff > 0 {
		return "+" + Pct(diff)
	}
	return Pct(diff)
}

// --- Entur -----------------------------------------------------------------------

// Trains is the T-BANE cell: the next two departures.
type Trains struct {
	Departures []Train // at most two
}

type Train struct {
	Time string // "11:48"
	Dot  string // "red" when delayed, "grey" without realtime data
}

// minLeadTime: a train leaving sooner than this can't be reached, so it
// isn't shown.
const minLeadTime = 2 * time.Minute

// BuildTrains lists the next two reachable departures. Entur is polled every
// minute (and its last answer kept if it fails), so trains that have left
// are skipped here.
func BuildTrains(trains []entur.Train, now time.Time) Trains {
	var out Trains
	for _, t := range trains {
		if len(out.Departures) == 2 {
			break
		}
		if t.Time.Sub(now) < minLeadTime {
			continue
		}
		dot := ""
		switch {
		case t.Delayed:
			dot = "red"
		case !t.Realtime:
			dot = "grey"
		}
		out.Departures = append(out.Departures, Train{Time: t.Time.In(tz.Oslo).Format("15:04"), Dot: dot})
	}
	return out
}
