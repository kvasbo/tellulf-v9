package views

import (
	"context"
	"strings"
	"testing"

	"github.com/kvasbo/tellulf-v9/internal/view"
)

// Without a recent reading there is no number, so no unit on its own either.
func TestStromDrawsThePowerOnlyWithAReading(t *testing.T) {
	render := func(s view.Strom) string {
		var b strings.Builder
		if err := Strom(s).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got := render(view.Strom{Label: "Hjemme", HasData: true, Now: "2,4"}); !strings.Contains(got, "2,4") || !strings.Contains(got, ">kW<") {
		t.Errorf("with a reading:\n%s", got)
	}
	if got := render(view.Strom{Label: "Hjemme", HasData: true}); strings.Contains(got, ">kW<") {
		t.Errorf("without a reading:\n%s", got)
	}
}
