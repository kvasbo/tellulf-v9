package tibber

import (
	"errors"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func nov6(h, m int) time.Time { return time.Date(2026, 11, 6, h, m, 0, 0, tz.Oslo) }

// Once an hour when it works. When it fails, sooner than the next hour, or a
// failure at midnight leaves the wrong month's total up for an hour, but with
// the backoff Tibber requires.
func TestMonthlyPace(t *testing.T) {
	trouble := errors.New("503")
	var p monthlyPace

	if !p.due(nov6(10, 30)) {
		t.Fatal("the first fetch should be due at once")
	}
	p.done(nov6(10, 30), nil)
	if p.due(nov6(10, 59)) {
		t.Error("fetched already this hour")
	}
	if !p.due(nov6(11, 0)) {
		t.Fatal("a new hour should be due")
	}

	p.done(nov6(11, 0), trouble)
	if p.due(nov6(11, 0)) {
		t.Error("should wait a little after a failure")
	}
	if !p.due(nov6(11, 1)) {
		t.Fatal("the first retry should come within a minute, not next hour")
	}

	for range 6 {
		p.done(nov6(11, 1), trouble)
	}
	if p.due(nov6(11, 2)) {
		t.Error("repeated failures should wait longer")
	}
	if !p.due(nov6(11, 1).Add(maxBackoff)) {
		t.Error("the wait should stay under the longest backoff")
	}

	p.done(nov6(11, 40), nil)
	if p.due(nov6(11, 41)) {
		t.Error("a success should end the retries for this hour")
	}
	if !p.due(nov6(12, 0)) {
		t.Fatal("back to once an hour after a success")
	}

	// A rejected token isn't to be retried, so it just waits for its hour.
	p.done(nov6(12, 0), errUnauthorized)
	if p.due(nov6(12, 30)) {
		t.Error("a rejected token should not be retried within the hour")
	}
}

// consumption is a node for the hour starting at from: kWh, costing 0.5 kr each.
func consumption(from time.Time, kwh float64) consumptionNode {
	cost := kwh * 0.5
	return consumptionNode{From: from, Consumption: &kwh, Cost: &cost}
}

// After midnight the history has yesterday, so a feed that hasn't delivered
// today must not add its last total to it: that day would count twice, and
// the month's share of the quota would be too high.
func TestMonthlyDoesNotCountYesterdaysLiveTotalTwice(t *testing.T) {
	yesterday := nov6(0, 0).AddDate(0, 0, -1)
	nodes := []consumptionNode{consumption(yesterday.Add(20*time.Hour), 10), consumption(yesterday.Add(21*time.Hour), 2)}

	tb := New(Config{})
	d := tb.data[Home]
	d.Timestamp = yesterday.Add(23*time.Hour + 50*time.Minute)
	d.AccumulatedConsumption, d.AccumulatedProduction, d.AccumulatedCost = 12, 3, 6

	tb.applyMonthly(Home, nodes, nov6(0, 1))

	if !near(d.MonthlyConsumption, 12) {
		t.Errorf("month: %v kWh, want 12", d.MonthlyConsumption)
	}
	if !near(d.MonthlyCost, 6) {
		t.Errorf("month: %v kr, want 6", d.MonthlyCost)
	}
	if d.AccumulatedConsumption != 0 || d.AccumulatedProduction != 0 || d.AccumulatedCost != 0 {
		t.Errorf("today's totals are yesterday's: %+v", d)
	}
}

func TestMonthlyAddsTodaysLiveTotal(t *testing.T) {
	nodes := []consumptionNode{consumption(nov6(0, 0).AddDate(0, 0, -1), 12)}
	tb := New(Config{})
	d := tb.data[Home]
	d.Timestamp = nov6(9, 59)
	d.AccumulatedConsumption, d.AccumulatedCost = 5, 2.5

	tb.applyMonthly(Home, nodes, nov6(10, 0))

	if !near(d.MonthlyConsumption, 17) || !near(d.MonthlyCost, 8.5) {
		t.Errorf("month: %v kWh, %v kr; want 17, 8.5", d.MonthlyConsumption, d.MonthlyCost)
	}
	if !near(d.AccumulatedConsumption, 5) {
		t.Errorf("today: %v kWh, want 5", d.AccumulatedConsumption)
	}
}
