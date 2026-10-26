package views

import (
	"context"
	"strings"
	"testing"

	"github.com/kvasbo/tellulf-v9/internal/view"
)

// between returns the markup from the first occurrence of start up to the
// next "</div>".
func between(html, start string) string {
	i := strings.Index(html, start)
	if i < 0 {
		return ""
	}
	rest := html[i:]
	if j := strings.Index(rest, "</div>"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// The next departure is the big value; the one after it gets the detail
// line on its own, with no separator and no direction.
func TestEnturPutsSecondDepartureOnItsOwnLine(t *testing.T) {
	var b strings.Builder
	trains := view.Trains{Departures: []view.Train{{Time: "10:02"}, {Time: "10:05", Dot: "red"}}}
	if err := Entur(trains).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	value, detail := between(html, `class="value next"`), between(html, `class="detail"`)
	if !strings.Contains(value, "10:02") || strings.Contains(value, "10:05") {
		t.Errorf("value line = %q, want only 10:02", value)
	}
	if !strings.Contains(detail, "10:05") || !strings.Contains(detail, "train-dot-red") {
		t.Errorf("detail line = %q, want 10:05 with a red dot", detail)
	}
	if strings.Contains(html, "·") || strings.Contains(html, "mot ") {
		t.Errorf("no separator or direction expected: %q", html)
	}

	b.Reset()
	if err := Entur(view.Trains{Departures: trains.Departures[:1]}).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), `class="detail"`) {
		t.Errorf("one departure should give no detail line: %q", b.String())
	}
}
