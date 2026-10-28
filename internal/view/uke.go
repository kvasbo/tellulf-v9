package view

import (
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/claude"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Uke is the week page: Claude's lines about the days ahead, beside the
// days themselves as the overview lists them.
type Uke struct {
	Lines []string // none without Claude
	Days  []Day
}

// BuildUke lays out the week Claude writes about: seven days from today,
// or from tomorrow in the evening (claude.WeekStart).
func BuildUke(cal calendar.Snapshot, daily map[string]weather.Daily, texts claude.Texts, now time.Time) Uke {
	now = now.In(tz.Oslo)
	u := Uke{Lines: texts.Week}
	start := claude.WeekStart(now)
	for i := range claude.WeekDays {
		date := start.AddDate(0, 0, i)
		ahead := daysBetween(now, date)
		d := buildDay(cal, daily, texts.Titles, date, ahead)
		if ahead == 0 {
			d.Label = "I dag"
		}
		u.Days = append(u.Days, d)
	}
	return u
}
