package mode

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := map[string]Mode{
		"strom":       Strom,
		"Strøm":       Strom,
		" /strom\n":   Strom,
		"KALENDER":    Kalender,
		"/kalender":   Kalender,
		"oversikt":    Oversikt,
		"/":           Oversikt,
		"":            Oversikt,
		"  ":          Oversikt,
		"/Oversikt  ": Oversikt,
		"vær":         Vaer,
		"/vaer":       Vaer,
		"VÆR":         Vaer,
	}
	for in, want := range cases {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"natt", "strom/", "//strom", "/strom?x=1"} {
		if got, ok := Parse(in); ok {
			t.Errorf("Parse(%q) = %q, want not ok", in, got)
		}
	}
}

// Any page but the overview goes back to it after two minutes.
func TestCurrentReturnsToTheOverview(t *testing.T) {
	since := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		m     Mode
		after time.Duration
		want  Mode
	}{
		{Strom, 0, Strom},
		{Strom, Timeout - time.Second, Strom},
		{Strom, Timeout, Oversikt},
		{Vaer, time.Hour, Oversikt},
		{Oversikt, time.Hour, Oversikt},
	}
	for _, c := range cases {
		if got := Current(c.m, since, since.Add(c.after)); got != c.want {
			t.Errorf("%s after %v: got %s, want %s", c.m, c.after, got, c.want)
		}
	}
}

func TestPath(t *testing.T) {
	for m, want := range map[Mode]string{Oversikt: "/", Strom: "/strom", Kalender: "/kalender", Vaer: "/vaer"} {
		if got := m.Path(); got != want {
			t.Errorf("%s.Path() = %q, want %q", m, got, want)
		}
	}
}
