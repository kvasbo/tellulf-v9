// Package view turns raw data into exactly what the templates print. All
// formatting and arithmetic happens here, so the templates stay dumb.
package view

import (
	"fmt"
	"strconv"
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

// --- Clock -----------------------------------------------------------------------

// Clock is the time, date and week as client.js prints them. The server puts
// them in the page so its first frame is complete, which also gives a page
// transition a clock to move; client.js keeps them current from there.
type Clock struct {
	Time string // "9:05"
	Date string // "Tirsdag 27. oktober"
	Week string // "uke 44"
}

func BuildClock(now time.Time) Clock {
	now = now.In(tz.Oslo)
	_, week := now.ISOWeek()
	return Clock{
		Time: fmt.Sprintf("%d:%02d", now.Hour(), now.Minute()),
		Date: longDate(now),
		Week: "uke " + strconv.Itoa(week),
	}
}

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

// Power is one bottom-row cell: the power drawn right now, under it the
// month's consumption and how much of the Norgespris quota it has used, and
// the price only once the quota is used up, since until then every kWh costs
// the same fixed price.
type Power struct {
	Label   string // "Strøm hjemme", "Hytta"
	HasData bool   // false until the live feed has delivered
	Power   string // "0,0 kW"
	Usage   string // "2345 kWh · 47 %"; just the kWh without a cap
	Price   string // "1,23 kr" past the cap or without one, "" under it
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
	p.Usage = KWh(d.MonthlyConsumption)
	if d.Cap > 0 {
		p.Usage += " · " + Pct(d.MonthlyConsumption/d.Cap*100)
		if d.MonthlyConsumption < d.Cap {
			return p
		}
	}
	price := d.EffectivePrice
	if price == 0 {
		price = d.CurrentPrice
	}
	p.Price = Kr(price)
	return p
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
