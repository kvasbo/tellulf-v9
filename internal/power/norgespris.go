package power

import (
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Norgespris: a fixed price per kWh up to a monthly cap per place, spot price
// above it. The fixed price changes over time; each kWh costs the price in
// effect when it was used.
type Norgespris struct {
	Prices   []PricePeriod // sorted by From
	HomeCap  float64       // kWh/month
	CabinCap float64
}

// PricePeriod is a Norgespris price that applies from a point in time until
// the next period starts.
type PricePeriod struct {
	From  time.Time
	Price float64 // kr/kWh including VAT, comparable to the spot price
}

const vat = 1.25

var defaultNorgespris = Norgespris{
	Prices: []PricePeriod{
		{From: time.Date(2025, 10, 1, 0, 0, 0, 0, tz.Oslo), Price: 0.40 * vat},
		{From: time.Date(2027, 1, 1, 0, 0, 0, 0, tz.Oslo), Price: 0.45 * vat},
	},
	HomeCap:  5000,
	CabinCap: 1000,
}

// PriceAt returns the fixed price in effect at t.
func (n Norgespris) PriceAt(t time.Time) float64 {
	price := n.Prices[0].Price
	for _, p := range n.Prices {
		if t.Before(p.From) {
			break
		}
		price = p.Price
	}
	return price
}

func (n Norgespris) Cap(p Place) float64 {
	if p == Home {
		return n.HomeCap
	}
	return n.CabinCap
}

// EffectivePrice is what a kWh used at time at costs, given this month's
// consumption so far.
func (n Norgespris) EffectivePrice(p Place, at time.Time, monthlyConsumption, spotPrice float64) float64 {
	if monthlyConsumption >= n.Cap(p) {
		return spotPrice
	}
	return n.PriceAt(at)
}

// AccumulatedCost prices hours (in chronological order), charging the fixed
// price until the cap is reached and the hour's spot price after. used is
// what the month had used before these hours, in kWh.
func (n Norgespris) AccumulatedCost(p Place, hours []Usage, spot func(time.Time) float64, used float64) float64 {
	limit := n.Cap(p)
	total := 0.0
	for _, h := range hours {
		subsidized := max(0, min(h.Consumption, limit-used))
		market := h.Consumption - subsidized
		total += subsidized * n.PriceAt(h.Start)
		if market > 0 {
			total += market * spot(h.Start)
		}
		used += h.Consumption
	}
	return total
}
