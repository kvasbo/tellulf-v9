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
	if err := Calendar(c, view.Sun{}, view.Stamps{}).Render(context.Background(), &b); err != nil {
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

// Sunrise and sunset sit in the I DAG header, and are left out on a day the
// sun doesn't rise or set.
func TestSunInTodayHeader(t *testing.T) {
	render := func(s view.Sun) string {
		var b strings.Builder
		if err := Calendar(view.Calendar{}, s, view.Stamps{}).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got := render(view.Sun{Rise: "07:23", Set: "16:37"}); !strings.Contains(got, `<span class="label sun">Sol 07:23 · 16:37</span>`) {
		t.Errorf("sun missing from header: %s", got)
	}
	if got := render(view.Sun{}); strings.Contains(got, "Sol") {
		t.Errorf("sun shown without times: %s", got)
	}
}
