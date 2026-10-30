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

// Claude's sentence sits right under the dinner, and is left out without
// one.
func TestSentenceUnderDinner(t *testing.T) {
	render := func(today view.Today) string {
		var b strings.Builder
		if err := Calendar(view.Calendar{Today: today}, view.Sun{}, view.Stamps{}).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	got := render(view.Today{Dinner: "Taco", Sentence: "Regn fra kl. 15", Notes: []string{"Turdag E"}})
	if !strings.Contains(got, `<div class="dinner">Taco</div><div class="note">Regn fra kl. 15</div><div class="note">Turdag E</div>`) {
		t.Errorf("sentence not under the dinner: %s", got)
	}
	if got := render(view.Today{Dinner: "Taco"}); strings.Contains(got, `class="note"`) {
		t.Errorf("an empty sentence is shown: %s", got)
	}
}

// The week page has Claude's lines and the days, which reuse the overview's
// day rows.
func TestUke(t *testing.T) {
	var b strings.Builder
	u := view.Uke{Lines: []string{"Barna kommer fredag kl. 16"}, Days: []view.Day{{Label: "I dag", Dinner: "Taco"}}}
	if err := Uke(u).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<p>Barna kommer fredag kl. 16</p>`, `<span class="weekday">I dag<span class="dot dot-away"></span></span> <span class="dinner">Taco</span>`} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %s in %s", want, b.String())
		}
	}
}
