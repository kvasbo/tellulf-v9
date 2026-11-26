package spotpris

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

type entry struct {
	NOK   float64 `json:"NOK_per_kWh"`
	Start string  `json:"time_start"`
	End   string  `json:"time_end"`
}

// serve answers path with entries, and 404 for anything else.
func serve(t *testing.T, path string, entries []entry) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(entries)
	}))
	t.Cleanup(srv.Close)
	c := New("NO1")
	c.base = srv.URL
	return c
}

func TestDayAddsVAT(t *testing.T) {
	c := serve(t, "/2026/11-26_NO1.json", []entry{
		{NOK: 2.0, Start: "2026-11-26T00:00:00+01:00", End: "2026-11-26T01:00:00+01:00"},
	})
	got, err := c.Day(context.Background(), time.Date(2026, 11, 26, 15, 0, 0, 0, tz.Oslo))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 26, 0, 0, 0, 0, tz.Oslo)
	if len(got) != 1 || !near(got[0].Total, 2.5) || !got[0].Start.Equal(want) {
		t.Errorf("got %+v", got)
	}
}

func TestDayAveragesQuarterHours(t *testing.T) {
	var entries []entry
	for i, nok := range []float64{1, 2, 3, 4} {
		start := time.Date(2026, 11, 26, 0, 15*i, 0, 0, tz.Oslo)
		entries = append(entries, entry{NOK: nok, Start: start.Format(time.RFC3339), End: start.Add(15 * time.Minute).Format(time.RFC3339)})
	}
	c := serve(t, "/2026/11-26_NO1.json", entries)
	got, err := c.Day(context.Background(), time.Date(2026, 11, 26, 0, 0, 0, 0, tz.Oslo))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !near(got[0].Total, 2.5*1.25) {
		t.Errorf("got %+v", got)
	}
}

func TestDayNotPublished(t *testing.T) {
	c := serve(t, "/2026/11-26_NO1.json", nil)
	_, err := c.Day(context.Background(), time.Date(2026, 11, 27, 0, 0, 0, 0, tz.Oslo))
	if !errors.Is(err, ErrNotPublished) {
		t.Errorf("got %v, want ErrNotPublished", err)
	}
}

// The day the clocks go back has 25 hours, two of them starting at 02:00.
func TestDayOnDSTDay(t *testing.T) {
	var entries []entry
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, tz.Oslo)
	for h := day; h.Before(day.AddDate(0, 0, 1)); h = h.Add(time.Hour) {
		entries = append(entries, entry{NOK: 1, Start: h.Format(time.RFC3339), End: h.Add(time.Hour).Format(time.RFC3339)})
	}
	if len(entries) != 25 {
		t.Fatalf("test setup: %d hours", len(entries))
	}
	c := serve(t, "/2026/10-25_NO1.json", entries)
	got, err := c.Day(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, p := range got {
		seen[p.Start.Unix()] = true
	}
	if len(got) != 25 || len(seen) != 25 {
		t.Errorf("got %d prices, %d distinct hours", len(got), len(seen))
	}
}
