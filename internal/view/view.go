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

type Power struct {
	Header        string
	HasData       bool
	Usage         string
	Cost          string
	Price         string
	MonthlyStatus string
	MonthlyCost   string
	PriceStatus   string
	PriceColor    string
	BgColor       string
	PowerDisplay  string
	PowerWidth    string
	MaxWidth      string
	AvgWidth      string
	MinWidth      string
	MinBgColor    string
}

const (
	green  = "#3ef0a4"
	orange = "#f0a43e"
	blue   = "#3ea4f0"
)

func BuildPower(d tibber.PowerData, place tibber.Place, now time.Time) Power {
	now = now.In(tz.Oslo)
	norgespris := d.Cap != 0
	underCap := norgespris && d.MonthlyConsumption < d.Cap

	p := Power{
		Header:        map[tibber.Place]string{tibber.Home: "Hjemme", tibber.Cabin: "Hytta"}[place],
		HasData:       !d.Timestamp.IsZero(),
		Usage:         Fixed(d.AccumulatedConsumption, 2) + " kWh",
		Cost:          Fixed(d.AccumulatedCost, 2) + " kr",
		MonthlyStatus: Fixed(d.MonthlyConsumption, 1) + " kWh",
		MonthlyCost:   Fixed(d.MonthlyCost, 0) + " kr",
		PriceStatus:   "Spotpris",
		PriceColor:    orange,
		BgColor:       blue,
		PowerDisplay:  Fixed(d.CurrentPower/1000, 1),
		MinBgColor:    blue + "33",
	}
	price := d.EffectivePrice
	if price == 0 {
		price = d.CurrentPrice
	}
	p.Price = Fixed(price, 2) + " kr"

	// 30 November is "Megajouleens dag": show energy in MJ.
	if now.Month() == time.November && now.Day() == 30 {
		p.Usage = Num(tz.Round(d.AccumulatedConsumption*3.6)) + " MJ"
	}

	if norgespris {
		// How far ahead or behind a straight-line use of the cap we are.
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz.Oslo)
		monthEnd := monthStart.AddDate(0, 1, 0)
		fraction := float64(now.Sub(monthStart)) / float64(monthEnd.Sub(monthStart))
		expected := math.Max(fraction*d.Cap, 1)
		diff := (d.MonthlyConsumption - expected) / expected * 100
		sign := ""
		if diff > 0 {
			sign = "+"
		}
		p.MonthlyStatus += " (" + sign + Fixed(diff, 0) + "%)"
	}
	if underCap {
		p.PriceStatus, p.PriceColor = "Norgespris", green
	}
	if d.CurrentPower < 0 {
		p.BgColor = green
	}

	// Bar widths are whole percent of 30 kW, so per-second watt jitter doesn't
	// produce new HTML every tick (which would defeat the SSE dedup).
	const maxPower = 30000.0 / 100
	minP := d.MinPower
	if d.MaxPowerProduction > 0 {
		minP = math.Min(d.MinPower, -d.MaxPowerProduction)
	}
	if minP < 0 {
		p.MinBgColor = green
	}
	p.PowerWidth = Num(tz.Round(math.Abs(d.CurrentPower / maxPower)))
	p.MaxWidth = Num(tz.Round(d.MaxPower / maxPower))
	p.AvgWidth = Num(tz.Round(d.AveragePower / maxPower))
	p.MinWidth = Num(tz.Round(math.Abs(minP / maxPower)))
	return p
}

// --- Entur -----------------------------------------------------------------------

type Train struct {
	Time        string
	Destination string
	Dot         string // "red" when delayed, "grey" without realtime data
}

// BuildTrains lists the next four departures. Entur is polled every minute
// (and its last answer kept if it fails), so trains that have left are
// skipped here.
func BuildTrains(trains []entur.Train, now time.Time) []Train {
	var out []Train
	for _, t := range trains {
		if len(out) == 4 {
			break
		}
		if t.Time.Before(now) {
			continue
		}
		dot := ""
		switch {
		case t.Delayed:
			dot = "red"
		case !t.Realtime:
			dot = "grey"
		}
		out = append(out, Train{Time: t.Time.In(tz.Oslo).Format("15:04"), Destination: t.Destination, Dot: dot})
	}
	return out
}
