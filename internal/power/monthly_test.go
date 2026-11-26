package power

import (
	"errors"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
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
	if !p.due(nov6(11, 1).Add(schedule.MaxBackoff)) {
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
	p.done(nov6(12, 0), tibber.ErrUnauthorized)
	if p.due(nov6(12, 30)) {
		t.Error("a rejected token should not be retried within the hour")
	}
}
