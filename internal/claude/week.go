package claude

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

const (
	// WeekDays is how many days the week ahead covers.
	WeekDays = 7
	// weekFromTomorrow is the hour from which the week starts tomorrow:
	// in the evening, today is done.
	weekFromTomorrow = 18
	// weekInterval is the least time between two weeks. The input only
	// changes with the calendar, the forecast and the day.
	weekInterval = 3 * time.Hour
	// weekMaxAge is how long the week stays up after its input was last
	// seen: past that, the calendar has changed under it.
	weekMaxAge = 2 * weekInterval
	// weekMaxLines and weekLineMaxLen bound what is accepted.
	weekMaxLines   = 6
	weekLineMaxLen = 140
)

const weekSystem = `Du skriver noen få linjer om dagene som kommer, til en hjemmeskjerm som henger på veggen hjemme hos Audun i Oslo. Ved siden av linjene viser skjermen hver dag med middag, temperatur og avtaler. Linjene skal derfor ikke gjenta avtalene én og én, men løfte fram det man må tenke på når man planlegger uka. Ta med det som finnes av dette, i denne rekkefølgen:
1. når barna kommer og drar
2. bursdager
3. dager uten planlagt middag (si hvilke)
4. dager eller kvelder med mye på en gang
5. tydelige værskifter i Oslo, og været på hytta når noen skal dit

Regler:
- Norsk bokmål. Én til fem linjer, hver en kort, naturlig setning om én ting, høyst 90 tegn, uten punktum til slutt.
- Si ukedagen («onsdag», «på torsdag»), ikke datoen. «I dag» og «i morgen» er greit.
- Skriv bare om det som skjer. Ingen linjer om at noe ikke skjer («ingen bursdager», «ingen barnebytter»); dager uten middag er unntaket. Skjer det lite, holder én linje.
- Skriv med egne ord, ikke siter titler fra kalenderen ordrett. Et navn i en tittel er som regel den avtalen gjelder, ikke et sted.
- Bare fakta fra dataene. Hvert tall og klokkeslett du skriver, må stå i dataene. Ikke gjett på hvem som skal hvor eller hvorfor.
- Klokkeslett som «kl. 16» eller «kl. 16:30», temperaturer som «8°», og «−» for minus. Ikke nevn regnsjanse under 50 %.
- Ingen innledning, hilsen, oppsummering, utropstegn eller emoji.`

type weekText struct {
	written
	Lines []string `json:"lines"`
}

// WeekStart is the first day the week ahead covers: today, and from 18:00
// tomorrow.
func WeekStart(now time.Time) time.Time {
	now = now.In(tz.Oslo)
	if now.Hour() >= weekFromTomorrow {
		now = now.AddDate(0, 0, 1)
	}
	return tz.StartOfDay(now)
}

// weekInput is what the week is written from: each day's calendar and
// forecast, and the cabin's on the days someone is there.
func weekInput(cal calendar.Snapshot, oslo, hytta map[string]weather.Daily, now time.Time) string {
	now = now.In(tz.Oslo)
	start := WeekStart(now)
	var b strings.Builder
	// The day, not the hour: the week is the same all day.
	b.WriteString("I dag er det " + longDate(now) + " " + strconv.Itoa(now.Year()) + ".\n")
	for i := range WeekDays {
		date := start.AddDate(0, 0, i)
		head := longDate(date)
		switch {
		case tz.SameDay(date, now):
			head += " (i dag)"
		case tz.SameDay(date, now.AddDate(0, 0, 1)):
			head += " (i morgen)"
		}
		b.WriteString("\n" + capitalize(head) + ":\n")
		describeDay(&b, cal, date, now)
		if f, ok := oslo[dateKey(date)]; ok {
			item(&b, "Været i Oslo: "+dayForecast(f))
		}
		if f, ok := hytta[dateKey(date)]; ok && atHytta(cal, date) {
			item(&b, "Været på hytta: "+dayForecast(f))
		}
	}
	return b.String()
}

// refreshWeek writes the week anew when the input has changed: at most
// every few hours, and at once when the week starts on a new day.
func (c *Claude) refreshWeek(ctx context.Context, cal calendar.Snapshot, now time.Time) {
	in := weekInput(cal, c.src.Daily(weather.Oslo), c.src.Daily(weather.Hytta), now)
	hash, start := hashOf(in), dateKey(WeekStart(now))

	c.mu.Lock()
	last := c.week.written
	if hash == last.Hash {
		c.week.Checked = now
	}
	c.mu.Unlock()
	if !c.weekJob.due(last, hash, start, now) {
		return
	}

	var answer struct {
		Linjer []string `json:"linjer"`
	}
	err := c.generate(ctx, request{
		name:   "week",
		system: weekSystem,
		user:   in,
		schema: object(map[string]any{"linjer": arrayOf(str)}),
		effort: anthropic.OutputConfigEffortMedium,
	}, &answer)
	var lines []string
	if err == nil {
		lines, err = checkWeek(answer.Linjer, in)
	}
	if err != nil {
		c.weekJob.failed(now)
		slog.Warn("claude: no week", "err", err, "retry_in", c.weekJob.retry)
		return
	}
	c.weekJob.succeeded(now)
	c.mu.Lock()
	c.week = weekText{written{Hash: hash, Period: start, Checked: now}, lines}
	c.dirty = true
	c.mu.Unlock()
	slog.Info("claude: week", "lines", strings.Join(lines, " | "))
}

// checkWeek tidies the lines and leaves out each one that is too long or
// has a number that isn't in the input. No lines is an answer (nothing
// worth saying); lines that are all left out is a failure.
func checkWeek(lines []string, in string) ([]string, error) {
	var out []string
	given := 0
	for _, l := range lines {
		l = tidy(l)
		if l != "" {
			given++
		}
		switch {
		case l == "":
		case utf8.RuneCountInString(l) > weekLineMaxLen:
			slog.Warn("claude: week line too long", "line", l)
		case !grounded(l, in):
			slog.Warn("claude: week line with a number that isn't in the input", "line", l)
		default:
			out = append(out, l)
		}
	}
	if given > 0 && len(out) == 0 {
		return nil, errors.New("no usable lines")
	}
	return out[:min(len(out), weekMaxLines)], nil
}
