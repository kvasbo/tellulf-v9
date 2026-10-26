// Package view turns raw data into exactly what the templates print. All
// formatting and arithmetic happens here, so the templates stay dumb.
package view

import (
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
// price per kWh and how much of the month's Norgespris quota is used.
type Power struct {
	Label   string // "Strøm hjemme", "Hytta"
	HasData bool   // false until the live feed has delivered
	Power   string // "0,0 kW"
	Detail  string // "0,50 kr · 27 %"; just the price without a cap
}

var placeLabels = map[tibber.Place]string{tibber.Home: "Strøm hjemme", tibber.Cabin: "Hytta"}

func BuildPower(d tibber.PowerData, place tibber.Place) Power {
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
		p.Detail += " · " + Pct(d.MonthlyConsumption/d.Cap*100)
	}
	return p
}

// --- Entur -----------------------------------------------------------------------

// Trains is the T-BANE cell: the next two departures and their direction.
type Trains struct {
	Departures []Train // at most two
	Direction  string  // "mot Bergkrystallen"; "" without departures
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
		if out.Direction == "" {
			out.Direction = "mot " + t.Destination
		}
		out.Departures = append(out.Departures, Train{Time: t.Time.In(tz.Oslo).Format("15:04"), Dot: dot})
	}
	return out
}
