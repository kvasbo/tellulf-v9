package view

import (
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/power"
)

// With a dead feed the last reading must not pass for the power right now.
// The month's use stays: it comes mostly from the history.
func TestBuildPowerHidesAnOldReading(t *testing.T) {
	now := oslo(27, 11, 30)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"just now", now, "1,2 kW"},
		{"9 minutes ago", now.Add(-9 * time.Minute), "1,2 kW"},
		{"11 minutes ago", now.Add(-11 * time.Minute), ""},
		{"yesterday", now.AddDate(0, 0, -1), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := power.PowerData{Timestamp: c.at, CurrentPower: 1200, MonthlyConsumption: 100, Cap: 5000}
			p := BuildPower(d, power.Home, now)
			if plain(p.Power) != c.want {
				t.Errorf("power %q, want %q", p.Power, c.want)
			}
			if !p.HasData || p.Usage == "" {
				t.Errorf("the month's use should stay: %+v", p)
			}
		})
	}
}

// Minutes after midnight the last reading is recent, but its totals are for
// the day before.
func TestBuildPowerHidesYesterdaysReadingJustAfterMidnight(t *testing.T) {
	d := power.PowerData{Timestamp: oslo(26, 23, 58), CurrentPower: 1200}
	if got := BuildPower(d, power.Home, oslo(27, 0, 1)).Power; got != "" {
		t.Errorf("power %q", got)
	}
}

func TestBuildStromHidesAnOldReading(t *testing.T) {
	now := oslo(27, 11, 30)

	d := stromDetails()
	d.Timestamp = now.Add(-9 * time.Minute)
	fresh := BuildStrom(d, power.Home, now)
	if fresh.Now == "" || fresh.Line1 == "" || fresh.Rows[0].Value == "" {
		t.Errorf("a recent reading should show: %+v", fresh)
	}

	d.Timestamp = now.Add(-11 * time.Minute)
	stale := BuildStrom(d, power.Home, now)
	if stale.Now != "" || stale.Line1 != "" || stale.Line2 != "" {
		t.Errorf("the power now should be gone: %q, %q, %q", stale.Now, stale.Line1, stale.Line2)
	}
	today := stale.Rows[0]
	if today.Value != "" || len(today.Details) != 0 {
		t.Errorf("without known hours today's totals should be gone: %q, %q", today.Value, today.Details)
	}
	if today.Chart == nil {
		t.Error("today's hours from the history should stay")
	}
	if stale.Rows[1].Value == "" || !stale.HasData {
		t.Errorf("the month should stay: %+v", stale.Rows[1])
	}
}

// Without a live feed, today is the known hours, saying how far they reach.
func TestBuildStromTodayFromHours(t *testing.T) {
	d := stromDetails()
	d.Timestamp = time.Time{}
	d.MonthKnown = true
	d.AccumulatedConsumption, d.AccumulatedCost = 8.5, 4.25
	d.TodayUntil = oslo(27, 11, 0)
	s := BuildStrom(d, power.Home, oslo(27, 11, 30))
	if !s.HasData || s.Now != "" || s.Line1 != "" {
		t.Errorf("got %+v", s)
	}
	today := s.Rows[0]
	if plain(today.Value) != "8,5 kWh" || len(today.Details) != 2 || today.Details[1] != "til kl. 11" {
		t.Errorf("today: %q, %q", today.Value, today.Details)
	}
}

// Just after midnight there are no hours yet today: the month still shows.
func TestBuildStromNoHoursYetToday(t *testing.T) {
	d := stromDetails()
	d.Timestamp = time.Time{}
	d.MonthKnown = true
	s := BuildStrom(d, power.Home, oslo(27, 0, 20))
	if !s.HasData || s.Rows[0].Value != "" || len(s.Rows[0].Details) != 0 || s.Rows[1].Value == "" {
		t.Errorf("got %+v", s)
	}
}

// The home without a live feed: the month and quota, but no power now.
func TestBuildPowerFromHoursOnly(t *testing.T) {
	d := power.PowerData{MonthKnown: true, MonthlyConsumption: 2345, Cap: 5000}
	p := BuildPower(d, power.Home, oslo(27, 11, 30))
	if !p.HasData || p.Power != "" || p.Usage == "" {
		t.Errorf("got %+v", p)
	}
}
