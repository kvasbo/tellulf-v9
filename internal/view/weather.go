package view

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/sun"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Weather is the left column's weather block: the temperature now, two
// sentences built from the hourly forecast, and six three-hour columns.
type Weather struct {
	Temperature string // "11°"; "" without a sensor and a forecast
	Line1       string // "Sol nå, skyet fra kl. 14"
	Line2       string // "Ned mot 4° i natt · regn fra kl. 02"
	Columns     []Column
}

// Column is one three-hour step of the forecast row.
type Column struct {
	Hour        string // "15", "00"
	Temperature string // "7°"; "" when unknown
	Rain        bool
	RainPx      string // bar thickness in px at 1080p, "3".."15"; "" when !Rain
	RainLevel   int    // 1 light, 2 moderate, 3 heavy; 0 when !Rain
}

// wet is the precipitation (mm in an hour) that counts as rain.
const wet = 0.1

// sensorMaxAge: an outdoor sensor that has been quiet longer than this is
// treated as missing, so a dead broker can't pin an old reading on screen.
const sensorMaxAge = 20 * time.Minute

// BuildWeather prefers a fresh outdoor sensor for the temperature now, then
// the forecast for the current hour. hours[0] is the hour now falls in.
func BuildWeather(r smarthouse.Readings, hours []weather.Hourly, now time.Time) Weather {
	w := Weather{Line1: line1(hours), Line2: line2(hours, now), Columns: columns(hours)}
	sensorFresh := r.TempOut != nil && (r.LastTempTime.IsZero() || now.Sub(r.LastTempTime) <= sensorMaxAge)
	switch {
	case sensorFresh:
		w.Temperature = Deg(*r.TempOut)
	case len(hours) > 0 && hours[0].AirTemperature != nil:
		w.Temperature = Deg(*hours[0].AirTemperature)
	}
	return w
}

// symbolWord turns a MET symbol code into the word the sentences use.
func symbolWord(symbol string) string {
	base, _, _ := strings.Cut(symbol, "_")
	switch {
	case strings.Contains(base, "thunder"):
		return "Torden"
	case strings.Contains(base, "sleet"):
		return "Sludd"
	case strings.Contains(base, "snow"):
		return "Snø"
	case strings.Contains(base, "showers"):
		return "Byger"
	case strings.Contains(base, "heavyrain"):
		return "Kraftig regn"
	case strings.Contains(base, "rain"):
		return "Regn"
	}
	switch base {
	case "clearsky":
		if strings.HasSuffix(symbol, "_night") {
			return "Klart"
		}
		return "Sol"
	case "fair":
		return "Lettskyet"
	case "partlycloudy":
		return "Delvis skyet"
	case "fog":
		return "Tåke"
	}
	return "Skyet"
}

// line1 names the weather now and the first change within twelve hours.
func line1(hours []weather.Hourly) string {
	if len(hours) == 0 {
		return ""
	}
	now := symbolWord(hours[0].Symbol)
	for i := 1; i < len(hours) && i <= 12; i++ {
		if w := symbolWord(hours[i].Symbol); w != now {
			return fmt.Sprintf("%s nå, %s fra kl. %02d", now, strings.ToLower(w), hours[i].Hour)
		}
	}
	return now + " nå og de neste timene"
}

// line2 gives the day's high (before 15:00) or the coming night's low, and
// when precipitation starts or stops.
func line2(hours []weather.Hourly, now time.Time) string {
	if len(hours) == 0 {
		return ""
	}
	var parts []string
	if t := tempPart(hours, now); t != "" {
		parts = append(parts, t)
	}
	if r := rainPart(hours); r != "" {
		parts = append(parts, r)
	}
	return strings.Join(parts, " · ")
}

func tempPart(hours []weather.Hourly, now time.Time) string {
	h0 := hours[0].Hour
	end := len(hours)
	night := now.In(tz.Oslo).Hour() >= 15
	if night {
		// Through 08 tomorrow.
		end = min(end, 24-h0+8+1)
	} else {
		// The rest of today: until the hour number wraps past midnight.
		for i := 1; i < len(hours); i++ {
			if hours[i].Hour < hours[i-1].Hour {
				end = i
				break
			}
		}
	}
	var best float64
	found := false
	for _, h := range hours[:end] {
		if h.AirTemperature == nil {
			continue
		}
		switch {
		case !found:
			best, found = *h.AirTemperature, true
		case night:
			best = math.Min(best, *h.AirTemperature)
		default:
			best = math.Max(best, *h.AirTemperature)
		}
	}
	if !found {
		return ""
	}
	if night {
		return "Ned mot " + Deg(best) + " i natt"
	}
	return "Opp mot " + Deg(best) + " i dag"
}

func rainPart(hours []weather.Hourly) string {
	n := min(len(hours), 19) // look at hours 1..18
	if hours[0].PrecipitationAmount >= wet {
		for i := 1; i < n; i++ {
			if hours[i].PrecipitationAmount < wet {
				return fmt.Sprintf("%s til kl. %02d", precipWord(hours[0].Symbol), hours[i].Hour)
			}
		}
		return ""
	}
	for i := 1; i < n; i++ {
		if hours[i].PrecipitationAmount >= wet {
			return fmt.Sprintf("%s fra kl. %02d", precipWord(hours[i].Symbol), hours[i].Hour)
		}
	}
	return ""
}

func precipWord(symbol string) string {
	switch {
	case strings.Contains(symbol, "snow"):
		return "snø"
	case strings.Contains(symbol, "sleet"):
		return "sludd"
	}
	return "regn"
}

// columns picks six three-hour steps, starting at the first hour divisible
// by three after the current one. A column's bar sums the precipitation of
// its three hours; its colour follows the wettest single hour.
func columns(hours []weather.Hourly) []Column {
	if len(hours) == 0 {
		return nil
	}
	k := 1
	for (hours[0].Hour+k)%3 != 0 {
		k++
	}
	var out []Column
	for i := k; i < len(hours) && len(out) < 6; i += 3 {
		h := hours[i]
		c := Column{Hour: fmt.Sprintf("%02d", h.Hour)}
		if h.AirTemperature != nil {
			c.Temperature = Deg(*h.AirTemperature)
		}
		var sum, peak float64
		for _, x := range hours[i:min(i+3, len(hours))] {
			sum += x.PrecipitationAmount
			peak = math.Max(peak, x.PrecipitationAmount)
		}
		if sum >= wet {
			c.Rain = true
			c.RainPx = Num(3 + tz.Round(12*math.Min(sum, 9)/9))
			switch {
			case peak < 0.5:
				c.RainLevel = 1
			case peak < 2:
				c.RainLevel = 2
			default:
				c.RainLevel = 3
			}
		}
		out = append(out, c)
	}
	return out
}

// Sun is the SOL cell: sunrise and sunset in Oslo.
type Sun struct{ Rise, Set string }

func BuildSun(now time.Time) Sun {
	rise, set, ok := sun.Times(now.In(tz.Oslo), osloLat, osloLon)
	if !ok {
		return Sun{}
	}
	return Sun{Rise: rise.Format("15:04"), Set: set.Format("15:04")}
}
