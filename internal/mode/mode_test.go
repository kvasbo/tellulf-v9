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
		"uke":         Uke,
		"/Uke":        Uke,
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

// Any page but the overview goes back to it after the timeout; a timeout of
// 0 keeps it.
func TestCurrentReturnsToTheOverview(t *testing.T) {
	since := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	const two = DefaultTimeout
	cases := []struct {
		m       Mode
		after   time.Duration
		timeout time.Duration
		want    Mode
	}{
		{Strom, 0, two, Strom},
		{Strom, two - time.Second, two, Strom},
		{Strom, two, two, Oversikt},
		{Vaer, time.Hour, two, Oversikt},
		{Oversikt, time.Hour, two, Oversikt},
		{Kalender, 9 * time.Minute, 10 * time.Minute, Kalender},
		{Kalender, 24 * time.Hour, 0, Kalender},
	}
	for _, c := range cases {
		if got := Current(c.m, since, since.Add(c.after), c.timeout); got != c.want {
			t.Errorf("%s after %v with timeout %v: got %s, want %s", c.m, c.after, c.timeout, got, c.want)
		}
	}
}

func TestPath(t *testing.T) {
	for m, want := range map[Mode]string{Oversikt: "/", Strom: "/strom", Kalender: "/kalender", Vaer: "/vaer", Uke: "/uke"} {
		if got := m.Path(); got != want {
			t.Errorf("%s.Path() = %q, want %q", m, got, want)
		}
	}
}
