package tibber

import "time"

// Norgespris: a fixed price per kWh up to a monthly cap per place, spot price
// above it.
type Norgespris struct {
	SubsidizedPrice float64 // kr/kWh
	HomeCap         float64 // kWh/month
	CabinCap        float64
	Start           time.Time
}

var defaultNorgespris = Norgespris{
	SubsidizedPrice: 0.5,
	HomeCap:         5000,
	CabinCap:        1000,
	Start:           time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
}

// NorgesprisActive reports whether the Norgespris scheme has started.
func NorgesprisActive() bool { return defaultNorgespris.Active() }

func (n Norgespris) Active() bool { return !time.Now().Before(n.Start) }

func (n Norgespris) Cap(p Place) float64 {
	if p == Home {
		return n.HomeCap
	}
	return n.CabinCap
}

// EffectivePrice is what the next kWh costs given this month's consumption.
func (n Norgespris) EffectivePrice(p Place, monthlyConsumption, spotPrice float64) float64 {
	if !n.Active() || monthlyConsumption >= n.Cap(p) {
		return spotPrice
	}
	return n.SubsidizedPrice
}

type hourlyUsage struct {
	Start       time.Time
	Price       float64 // spot price per kWh, derived from cost/consumption
	Consumption float64
}

// AccumulatedCost prices a month's hours (in chronological order), charging
// the subsidized price until the cap is reached and spot price after.
func (n Norgespris) AccumulatedCost(p Place, hours []hourlyUsage) float64 {
	total := 0.0
	if !n.Active() {
		for _, h := range hours {
			total += h.Price * h.Consumption
		}
		return total
	}
	limit := n.Cap(p)
	used := 0.0
	for _, h := range hours {
		if used < limit {
			subsidized := min(h.Consumption, limit-used)
			market := max(0, h.Consumption-subsidized)
			total += subsidized*n.SubsidizedPrice + market*h.Price
		} else {
			total += h.Consumption * h.Price
		}
		used += h.Consumption
	}
	return total
}
