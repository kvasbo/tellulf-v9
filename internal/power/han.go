package power

import (
	"errors"
	"log/slog"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/han"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// hanGap: the energy between two readings is only counted when they are this
// close. Across a longer gap the power in between is unknown.
const hanGap = time.Minute

// hanHome is the home's live state from the HAN reader.
type hanHome struct {
	last     time.Time // the latest reading
	power    float64   // W in it, import minus export
	export   float64   // W, from the latest frame that had it
	today    float64   // kWh counted today since the first reading
	min, max float64   // W today
}

// HAN takes a message from the home's HAN reader (a Tibber Pulse publishing
// to MQTT). Messages that aren't meter frames, like the Pulse's status, are
// skipped.
func (p *Power) HAN(payload []byte) {
	p.han(payload, time.Now())
}

func (p *Power) han(payload []byte, now time.Time) {
	r, err := han.Decode(payload)
	if errors.Is(err, han.ErrNotFrame) {
		return
	}
	if err != nil {
		slog.Debug("HAN frame skipped", "err", err)
		return
	}
	w, ok := r[han.ActivePowerImport]
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h := &p.hanHome
	// Export only comes every 10 seconds; the frames between have import alone.
	if e, ok := r[han.ActivePowerExport]; ok {
		h.export = e
	}
	w -= h.export
	if !tz.SameDay(h.last, now) {
		h.today, h.min, h.max = 0, w, w
	} else if gap := now.Sub(h.last); gap < hanGap {
		h.today += h.power * gap.Hours() / 1000
	}
	h.min, h.max = min(h.min, w), max(h.max, w)
	h.last, h.power = now, w
	p.marks[Home].addCounter(now, h.today)
	p.live[Home] = PowerData{
		Timestamp:              now,
		AccumulatedConsumption: h.today,
		CurrentPower:           w,
		MinPower:               h.min,
		MaxPower:               h.max,
	}
}
