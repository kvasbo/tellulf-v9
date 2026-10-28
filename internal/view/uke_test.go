package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/claude"
)

// The week page lists the seven days Claude wrote about: from today, and
// from tomorrow in the evening.
func TestUkeDays(t *testing.T) {
	lines := []string{"Barna kommer fredag kl. 16"}
	snap := calendar.Snapshot{Dinners: []calendar.Event{fullDay("Taco", mon)}}
	labels := func(u Uke) []string {
		var out []string
		for _, d := range u.Days {
			out = append(out, d.Label)
		}
		return out
	}

	u := BuildUke(snap, nil, claude.Texts{Week: lines}, calNow)
	if !slices.Equal(u.Lines, lines) {
		t.Errorf("lines = %q", u.Lines)
	}
	want := []string{"I dag", "Tirsdag", "Onsdag", "Torsdag", "Fredag", "Lørdag", "Søndag"}
	if got := labels(u); !slices.Equal(got, want) {
		t.Errorf("labels = %q, want %q", got, want)
	}
	if u.Days[0].Dinner != "Taco" {
		t.Errorf("today's dinner = %q", u.Days[0].Dinner)
	}

	u = BuildUke(snap, nil, claude.Texts{}, mon.Add(19*time.Hour))
	want = []string{"Tirsdag", "Onsdag", "Torsdag", "Fredag", "Lørdag", "Søndag", "Mandag 2."}
	if got := labels(u); !slices.Equal(got, want) || u.Lines != nil {
		t.Errorf("evening: labels = %q, lines = %q; want %q and none", got, u.Lines, want)
	}
}
