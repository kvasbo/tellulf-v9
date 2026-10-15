package view

import (
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/entur"
)

func TestBuildTrainsSkipsDeparted(t *testing.T) {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	var trains []entur.Train
	for _, m := range []int{-3, -1, 2, 5, 9, 12, 15} {
		trains = append(trains, entur.Train{Time: now.Add(time.Duration(m) * time.Minute), Realtime: true})
	}
	got := BuildTrains(trains, now)
	if len(got) != 4 || got[0].Time != "10:02" || got[3].Time != "10:12" {
		t.Errorf("got %+v, want the four from 10:02", got)
	}
}
