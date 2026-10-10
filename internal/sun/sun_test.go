package sun

import (
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Reference values from sunrise-sunset-js (NREL SPA), which the TypeScript
// version used, for Slemdal (59.9508, 10.6847).
func TestTimesMatchReference(t *testing.T) {
	cases := []struct{ date, rise, set string }{
		{"2026-01-01", "2026-01-01T08:19:02Z", "2026-01-01T14:22:49Z"},
		{"2026-03-21", "2026-03-21T05:16:10Z", "2026-03-21T17:34:02Z"},
		{"2026-06-21", "2026-06-21T01:53:33Z", "2026-06-21T20:44:34Z"},
		{"2026-09-23", "2026-09-23T05:03:38Z", "2026-09-23T17:14:17Z"},
		{"2026-12-21", "2026-12-21T08:18:42Z", "2026-12-21T14:11:54Z"},
	}
	const tolerance = 10 * time.Second
	for _, c := range cases {
		day, _ := time.ParseInLocation("2006-01-02", c.date, tz.Oslo)
		rise, set, ok := Times(day, 59.9508, 10.6847)
		if !ok {
			t.Fatalf("%s: no sunrise/sunset", c.date)
		}
		wantRise, _ := time.Parse(time.RFC3339, c.rise)
		wantSet, _ := time.Parse(time.RFC3339, c.set)
		if d := rise.Sub(wantRise).Abs(); d > tolerance {
			t.Errorf("%s sunrise off by %v (got %s)", c.date, d, rise)
		}
		if d := set.Sub(wantSet).Abs(); d > tolerance {
			t.Errorf("%s sunset off by %v (got %s)", c.date, d, set)
		}
	}
}

func TestPolarDaysReportNotOK(t *testing.T) {
	midsummer := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	if _, _, ok := Times(midsummer, 78.22, 15.65); ok { // Longyearbyen
		t.Error("expected midnight sun at Longyearbyen")
	}
}
