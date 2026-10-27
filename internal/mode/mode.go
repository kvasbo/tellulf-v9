// Package mode names the pages the screen can show. A message on MQTT picks
// one, and every open screen navigates to its URL.
package mode

import "strings"

type Mode string

const (
	Oversikt Mode = "oversikt" // the Natt layout
	Strom    Mode = "strom"    // Tibber in detail
	Kalender Mode = "kalender" // the next two weeks in detail
	Vaer     Mode = "vaer"     // the weather in detail
)

var paths = map[Mode]string{Oversikt: "/", Strom: "/strom", Kalender: "/kalender", Vaer: "/vaer"}

// ascii spells the Norwegian letters the way the paths do.
var ascii = strings.NewReplacer("ø", "o", "æ", "ae", "å", "a")

// Path is the page's URL path: "/", "/strom", "/kalender".
func (m Mode) Path() string { return paths[m] }

// Parse accepts a mode's name or its path, in any case and with spaces
// around: "strom", "Strøm", "/kalender", "vær". Empty means the overview.
func Parse(s string) (Mode, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "/")
	s = ascii.Replace(s)
	if s == "" {
		return Oversikt, true
	}
	m := Mode(s)
	_, ok := paths[m]
	return m, ok
}
