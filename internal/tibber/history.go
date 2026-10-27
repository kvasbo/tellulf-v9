package tibber

import (
	"maps"
	"slices"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Usage is the energy used in one hour or one day.
type Usage struct {
	Start       time.Time
	Consumption float64 // kWh
}

// Details is everything the power page shows for one place.
type Details struct {
	PowerData
	Hours      []Usage      // today's hours in order; an hour without data is missing
	Days       []Usage      // this month's days before today, from the consumption history
	Prices     []PricePoint // today, and tomorrow once Tibber has published it
	Norgespris float64      // the fixed price now, kr/kWh
}

// Details returns a copy of everything known about p.
func (t *Tibber) Details(p Place) Details {
	now := time.Now()
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Details{
		PowerData:  *t.data[p],
		Hours:      mergeHours(t.historyToday[p], t.live[p].hours(now), now),
		Days:       t.days[p], // replaced, never changed in place
		Prices:     t.prices[p],
		Norgespris: t.norgespris.PriceAt(now),
	}
}

// liveHours turns the live feed's running total into hourly consumption:
// it marks the total at every hour boundary it sees. The history has
// today's hours too, but an hour or two late; these fill the gap.
type liveHours struct {
	marks   map[time.Time]float64 // hour start → today's total then, kWh
	last    time.Time             // the latest measurement
	lastAcc float64               // and today's total in it
}

// markGap: a boundary is only marked when the measurement before it is this
// close. After a longer gap in the feed the total at the boundary is
// unknown, and using an older one would pile earlier hours onto this one.
const markGap = time.Minute

// add records a measurement of today's total consumption (kWh since
// midnight) taken at now.
func (l *liveHours) add(now time.Time, acc float64) {
	hour := now.Truncate(time.Hour) // Oslo's offset is whole hours
	if !tz.SameDay(l.last, now) {
		l.marks = map[time.Time]float64{}
	}
	if !l.last.IsZero() && l.last.Before(hour) && hour.Sub(l.last) < markGap {
		start := l.lastAcc
		if !tz.SameDay(l.last, now) {
			start = 0 // the total starts over at midnight
		}
		l.marks[hour] = start
	}
	l.last, l.lastAcc = now, acc
}

// hours is the consumption in each hour today with a mark at its start:
// up to the next mark, or for the hour now, up to the latest total.
func (l *liveHours) hours(now time.Time) []Usage {
	if l == nil || !tz.SameDay(l.last, now) {
		return nil
	}
	var out []Usage
	for _, start := range slices.SortedFunc(maps.Keys(l.marks), time.Time.Compare) {
		end, ok := l.marks[start.Add(time.Hour)]
		if !ok {
			if !start.Equal(l.last.Truncate(time.Hour)) {
				continue // the feed dropped before the hour ended
			}
			end = l.lastAcc
		}
		out = append(out, Usage{Start: start, Consumption: max(0, end-l.marks[start])})
	}
	return out
}

// mergeHours combines today's hours from the history with those measured
// live. The history wins where both have an hour; hours from another day
// are dropped. Hours are matched by Unix time, since the history's times
// carry Tibber's zone and the live ones ours.
func mergeHours(history, live []Usage, now time.Time) []Usage {
	byHour := map[int64]float64{}
	for _, list := range [][]Usage{live, history} {
		for _, u := range list {
			if tz.SameDay(u.Start, now) {
				byHour[u.Start.Unix()] = u.Consumption
			}
		}
	}
	out := make([]Usage, 0, len(byHour))
	for _, start := range slices.Sorted(maps.Keys(byHour)) {
		out = append(out, Usage{Start: time.Unix(start, 0).In(tz.Oslo), Consumption: byHour[start]})
	}
	return out
}

// dayTotals sums hours (in order) into one Usage per Oslo day.
func dayTotals(hours []hourlyUsage) []Usage {
	var out []Usage
	for _, h := range hours {
		day := tz.StartOfDay(h.Start)
		if n := len(out); n > 0 && out[n-1].Start.Equal(day) {
			out[n-1].Consumption += h.Consumption
			continue
		}
		out = append(out, Usage{Start: day, Consumption: h.Consumption})
	}
	return out
}

// historyToday picks today's known hours from the consumption history.
func historyToday(nodes []consumptionNode, now time.Time) []Usage {
	var out []Usage
	for _, n := range nodes {
		if n.Consumption != nil && tz.SameDay(n.From, now) {
			out = append(out, Usage{Start: n.From, Consumption: *n.Consumption})
		}
	}
	return out
}
