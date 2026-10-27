package mode

import "testing"

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

func TestPath(t *testing.T) {
	for m, want := range map[Mode]string{Oversikt: "/", Strom: "/strom", Kalender: "/kalender", Vaer: "/vaer"} {
		if got := m.Path(); got != want {
			t.Errorf("%s.Path() = %q, want %q", m, got, want)
		}
	}
}
