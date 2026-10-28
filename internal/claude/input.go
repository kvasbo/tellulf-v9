package claude

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// The input is plain Norwegian, written the way the prompts talk about it.
// Everything in it is something the screen also knows; a text may not bring
// in a number that isn't here (see grounded).

// weekdays is indexed by time.Weekday.
var weekdays = [...]string{"søndag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag"}

// months is indexed by time.Month - 1.
var months = [...]string{"januar", "februar", "mars", "april", "mai", "juni", "juli",
	"august", "september", "oktober", "november", "desember"}

// longDate is "tirsdag 28. oktober".
func longDate(t time.Time) string {
	t = t.In(tz.Oslo)
	return weekdays[t.Weekday()] + " " + strconv.Itoa(t.Day()) + ". " + months[t.Month()-1]
}

// capitalize upper-cases the first letter: "tirsdag" → "Tirsdag".
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// nowLine says what day and hour it is: "Nå: tirsdag 28. oktober 2026, kl. 07".
func nowLine(now time.Time) string {
	return fmt.Sprintf("Nå: %s %d, kl. %02d\n", longDate(now), now.Year(), now.Hour())
}

func item(b *strings.Builder, s string) { b.WriteString("- " + s + "\n") }

// describeDay lists what the calendar has on date: where the kids are, the
// dinner, birthdays and the events, leaving out what is over by now and the
// events the screen dims because the kids are away.
func describeDay(b *strings.Builder, cal calendar.Snapshot, date, now time.Time) {
	n := b.Len()
	if kids := kidsLine(cal, date); kids != "" {
		item(b, "Barna: "+kids)
	}
	dinners := cal.DinnersForDate(date)
	for _, d := range dinners {
		item(b, "Middag: "+d.DisplayTitle)
	}
	if len(dinners) == 0 && len(cal.Dinners) > 0 {
		item(b, "Middag: ingen planlagt")
	}
	for _, e := range cal.BirthdaysForDate(date) {
		if name, age, ok := e.Birthday(date.Year()); ok {
			item(b, "Bursdag: "+name+" fyller "+strconv.Itoa(age))
		} else {
			item(b, "Bursdag: "+e.Title)
		}
	}
	events := cal.EventsForDate(date)
	slices.SortStableFunc(events, func(a, b calendar.EnrichedEvent) int { return a.Start.Compare(b.Start) })
	for _, e := range events {
		if e.Faded || (!e.FullDay && e.End.Before(now)) {
			continue
		}
		item(b, eventLine(e))
	}
	if b.Len() == n {
		item(b, "ingenting i kalenderen")
	}
}

// kidsLine says where the kids are on date, seen from the screen's home;
// "" without the kids' calendar.
func kidsLine(cal calendar.Snapshot, date time.Time) string {
	if len(cal.Barneuker) == 0 {
		return ""
	}
	switch cal.KidsStatusForDate(date) {
	case calendar.KidsFull:
		return "her hele dagen"
	case calendar.KidsArriving:
		return fmt.Sprintf("kommer kl. %d, fra Hanne", calendar.HandoverHour)
	case calendar.KidsLeaving:
		return fmt.Sprintf("drar til Hanne kl. %d", calendar.HandoverHour)
	}
	return "hos Hanne"
}

// eventLine is "09:00–10:00 Tannlege (Storgata 1)", "Hele dagen: Høstferie
// (fra mandag 26. oktober til fredag 30. oktober)".
func eventLine(e calendar.EnrichedEvent) string {
	place := ""
	if p := shortPlace(e.Location); p != "" {
		place = " (" + p + ")"
	}
	if e.FullDay {
		span := ""
		if !tz.SameDay(e.Start, e.End) {
			span = " (fra " + longDate(e.Start) + " til " + longDate(e.End) + ")"
		}
		return "Hele dagen: " + e.Title + span + place
	}
	t := e.DisplayTime
	var when string
	switch e.DayType {
	case calendar.FirstDay:
		when = "fra " + t.Start + ":"
	case calendar.LastDay:
		when = "til " + t.End + ":"
	case calendar.MiddleDay:
		when = "hele dagen:"
	default:
		when = t.Start + "–" + t.End
	}
	return when + " " + e.Title + place
}

// shortPlace keeps the first part of a location, which names the place.
func shortPlace(loc string) string {
	first, _, _ := strings.Cut(loc, ",")
	first, _, _ = strings.Cut(first, "\n")
	return strings.TrimSpace(first)
}

// atHytta reports whether an event on date is at the cabin, so its
// forecast belongs with the day.
func atHytta(cal calendar.Snapshot, date time.Time) bool {
	for _, e := range cal.EventsForDate(date) {
		if !e.Faded && strings.Contains(strings.ToLower(e.Title), "hytta") {
			return true
		}
	}
	return false
}

// hourLine is "kl. 15: 7°, rain, 0,8 mm nedbør". The weather is MET's
// symbol code, which Claude reads fine.
func hourLine(h weather.Hourly) string {
	s := fmt.Sprintf("kl. %02d: ", h.Hour)
	if h.AirTemperature != nil {
		s += deg(*h.AirTemperature) + ", "
	}
	symbol, _, _ := strings.Cut(h.Symbol, "_") // "_day", "_night"
	s += symbol
	if h.PrecipitationAmount >= 0.1 {
		s += ", " + strings.Replace(strconv.FormatFloat(h.PrecipitationAmount, 'f', 1, 64), ".", ",", 1) + " mm nedbør"
	}
	return s
}

// dayForecast is "2° til 6°, 60 % sjanse for regn".
func dayForecast(f weather.Daily) string {
	return fmt.Sprintf("%s til %s, %d %% sjanse for regn", deg(f.MinTemp), deg(f.MaxTemp), int(tz.Round(f.LightRainProbability)))
}

// deg is a temperature in whole degrees: "7°", "−3°".
func deg(v float64) string {
	n := int(tz.Round(v))
	if n < 0 {
		return "−" + strconv.Itoa(-n) + "°"
	}
	return strconv.Itoa(n) + "°"
}
