package tibber

import (
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func osloAt(d, h, m, s int) time.Time { return time.Date(2026, 10, d, h, m, s, 0, tz.Oslo) }

func usages(u []Usage) map[int]float64 {
	out := map[int]float64{}
	for _, x := range u {
		out[x.Start.Hour()] = x.Consumption
	}
	return out
}

// The live feed marks the running total at each hour boundary it crosses;
// an hour's use is the difference to the next mark, and the hour now runs
// to the latest total.
func TestLiveHoursAtBoundaries(t *testing.T) {
	var l liveHours
	l.add(osloAt(27, 9, 40, 0), 10)   // started mid-hour: 09 is unknown
	l.add(osloAt(27, 9, 59, 55), 11)  // just before 10:00
	l.add(osloAt(27, 10, 0, 5), 11.1) // marks 10:00 at 11
	l.add(osloAt(27, 10, 59, 58), 13) //
	l.add(osloAt(27, 11, 0, 2), 13.2) // marks 11:00 at 13
	l.add(osloAt(27, 11, 20, 0), 14.5)

	got := usages(l.hours(osloAt(27, 11, 20, 1)))
	want := map[int]float64{10: 2, 11: 1.5}
	if len(got) != len(want) || !near(got[10], 2) || !near(got[11], 1.5) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// After a gap in the feed the total at the boundary is unknown, so the hour
// stays empty instead of taking the hours it missed.
func TestLiveHoursSkipsBoundaryAfterGap(t *testing.T) {
	var l liveHours
	l.add(osloAt(27, 9, 59, 50), 5)
	l.add(osloAt(27, 10, 0, 1), 5) // marks 10:00
	l.add(osloAt(27, 10, 30, 0), 6)
	// The feed is gone 10:30–12:10.
	l.add(osloAt(27, 12, 10, 0), 9)
	l.add(osloAt(27, 12, 40, 0), 10)

	got := usages(l.hours(osloAt(27, 12, 40, 0)))
	if len(got) != 0 {
		t.Errorf("got %v, want nothing: 10 never ended in the feed, 12 had no start", got)
	}
}

// The running total starts over at midnight, so the first hour of the day
// counts from zero, and yesterday's hours are gone.
func TestLiveHoursAtMidnight(t *testing.T) {
	var l liveHours
	l.add(osloAt(26, 22, 59, 59), 30)
	l.add(osloAt(26, 23, 0, 3), 30.1)
	l.add(osloAt(26, 23, 59, 58), 32)
	l.add(osloAt(27, 0, 0, 2), 0.01)
	l.add(osloAt(27, 0, 45, 0), 0.8)

	got := usages(l.hours(osloAt(27, 0, 45, 0)))
	if len(got) != 1 || !near(got[0], 0.8) {
		t.Errorf("got %v, want just 00 with 0.8", got)
	}
	if got := l.hours(osloAt(28, 9, 0, 0)); got != nil {
		t.Errorf("a day without measurements has no hours, got %v", got)
	}
}

// The history wins where it has an hour; the live feed fills the hours it
// hasn't caught up with. Other days are dropped.
func TestMergeHours(t *testing.T) {
	now := osloAt(27, 11, 30, 0)
	history := []Usage{
		{Start: osloAt(26, 23, 0, 0), Consumption: 9},
		{Start: osloAt(27, 8, 0, 0), Consumption: 1},
		{Start: osloAt(27, 9, 0, 0), Consumption: 2},
	}
	// The live feed's times are in another zone than the history's.
	live := []Usage{
		{Start: osloAt(27, 9, 0, 0).UTC(), Consumption: 2.5},
		{Start: osloAt(27, 10, 0, 0), Consumption: 3},
		{Start: osloAt(27, 11, 0, 0), Consumption: 0.5},
	}
	got := mergeHours(history, live, now)
	want := []float64{1, 2, 3, 0.5}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, u := range got {
		if u.Start.Hour() != 8+i || !near(u.Consumption, want[i]) {
			t.Errorf("hour %d: got %+v, want %v at %d", i, u, want[i], 8+i)
		}
	}
}

func TestDayTotals(t *testing.T) {
	hours := []hourlyUsage{
		{Start: osloAt(1, 0, 0, 0), Consumption: 1},
		{Start: osloAt(1, 23, 0, 0), Consumption: 2},
		{Start: osloAt(2, 0, 0, 0), Consumption: 4},
	}
	got := dayTotals(hours)
	if len(got) != 2 || !got[0].Start.Equal(osloAt(1, 0, 0, 0)) || got[0].Consumption != 3 ||
		!got[1].Start.Equal(osloAt(2, 0, 0, 0)) || got[1].Consumption != 4 {
		t.Errorf("got %+v", got)
	}
}

func TestHistoryTodayKeepsKnownHoursOfToday(t *testing.T) {
	nodes := []consumptionNode{
		{From: osloAt(26, 23, 0, 0), Consumption: ptr(9)},
		{From: osloAt(27, 0, 0, 0), Consumption: ptr(1)},
		{From: osloAt(27, 1, 0, 0), Consumption: nil},
	}
	got := historyToday(nodes, osloAt(27, 9, 0, 0))
	if len(got) != 1 || got[0].Consumption != 1 {
		t.Errorf("got %+v", got)
	}
}

// From 13:00 a cached list without tomorrow counts as missing, so the
// prices are fetched again.
func TestMissingTomorrow(t *testing.T) {
	tb := New(Config{})
	tb.prices[Home] = []PricePoint{{Start: osloAt(27, 0, 0, 0), Total: 1}}
	if tb.missingTomorrow(Home, osloAt(27, 12, 59, 0)) {
		t.Error("before 13:00 tomorrow isn't expected yet")
	}
	if !tb.missingTomorrow(Home, osloAt(27, 13, 0, 0)) {
		t.Error("from 13:00 a list without tomorrow is missing it")
	}
	tb.prices[Home] = append(tb.prices[Home], PricePoint{Start: osloAt(28, 0, 0, 0), Total: 1})
	if tb.missingTomorrow(Home, osloAt(27, 15, 0, 0)) {
		t.Error("tomorrow is cached")
	}
}

// Details copies the live numbers and the history, and today's live hours.
func TestDetailsIncludesProductionAndLiveHours(t *testing.T) {
	tb := New(Config{})
	tb.onMeasurement(Cabin, liveMeasurement{AccumulatedConsumption: 4, AccumulatedProduction: 1.5})
	d := tb.Details(Cabin)
	if d.AccumulatedProduction != 1.5 || d.AccumulatedConsumption != 2.5 || d.Norgespris == 0 {
		t.Errorf("got %+v", d)
	}
}
