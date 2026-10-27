// Package mode names the pages the screen can show. A message on MQTT picks
// one, and every open screen navigates to its URL; after Timeout they all go
// back to the overview.
package mode

import (
	"strings"
	"time"
)

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

// Timeout is how long a page other than the overview stays up.
const Timeout = 2 * time.Minute

// Path is the page's URL path: "/", "/strom", "/kalender", "/vaer".
func (m Mode) Path() string { return paths[m] }

// Current is the page to show at now when m was picked at since: m until it
// has been up for Timeout, then the overview.
func Current(m Mode, since, now time.Time) Mode {
	if m != Oversikt && now.Sub(since) >= Timeout {
		return Oversikt
	}
	return m
}

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
