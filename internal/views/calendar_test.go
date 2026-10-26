package views

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/view"
)

// Each kids status gets its own dot: filled when the kids are here, a ring
// when they're away, and the right or left half on handover days.
func TestKidsDotPerStatus(t *testing.T) {
	c := view.Calendar{
		Today: view.Today{Kids: calendar.KidsFull},
		Days: []view.Day{
			{Kids: calendar.KidsAway},
			{Kids: calendar.KidsArriving},
			{Kids: calendar.KidsLeaving},
		},
	}
	var b strings.Builder
	if err := Calendar(c, view.Stamps{}).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range regexp.MustCompile(`class="dot (dot-[a-z]+)"`).FindAllStringSubmatch(b.String(), -1) {
		got = append(got, m[1])
	}
	want := []string{"dot-full", "dot-away", "dot-arriving", "dot-leaving"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
