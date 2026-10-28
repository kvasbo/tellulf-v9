package claude

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

const (
	// todayInterval is the least time between two sentences. The input
	// names the hour, so a new one is written about once an hour.
	todayInterval = time.Hour
	// todayMaxAge is how long a sentence stays up after its input was last
	// seen: past that, it may talk about what has already happened.
	todayMaxAge = 90 * time.Minute
	// todayMaxLen is the longest sentence accepted, in characters: two
	// lines in I DAG.
	todayMaxLen = 100
	// todayHours is how far ahead the hourly forecast goes in the input.
	todayHours = 24
)

const todaySystem = `Du skriver én kort setning til en hjemmeskjerm som henger på veggen hjemme hos Audun i Oslo. Skjermen er rolig og saklig. Den viser allerede klokka, været nå, når nedbøren starter eller stopper, dagens høyeste (eller nattens laveste) temperatur, dagens middag, bursdager og avtalene med klokkeslett.

Setningen din står under middagen. Den skal bare si noe skjermen ikke sier av seg selv: en sammenheng mellom to ting i dataene. For eksempel:
- regn eller kulde akkurat når noe skal skje ute, eller når barna kommer eller drar
- en hyttetur og været på hytta
- noe tidlig i morgen som må forberedes i kveld
- en bursdag i morgen

Regler:
- Norsk bokmål, én setning, høyst 80 tegn, uten punktum til slutt.
- Skriv med egne ord, ikke siter titler fra kalenderen ordrett. Et navn i en tittel er som regel den avtalen gjelder, ikke et sted.
- Bare fakta fra dataene. Hvert tall og klokkeslett du skriver, må stå i dataene. Ikke gjett på hvem som skal hvor eller hvorfor.
- Klokkeslett som «kl. 16» eller «kl. 16:30», temperaturer som «8°», og «−» for minus.
- Ingen hilsen, ingen utropstegn, ingen emoji.
- Finnes det ingen slik sammenheng, svar med tom setning. Tom er bedre enn en setning som gjentar skjermen.
- Etter kl. 18 kan setningen handle om i morgen.`

type todayText struct {
	written
	Text string `json:"text"`
}

// todayInput is what the sentence is written from: the hour now, today
// and tomorrow in the calendar, the next hours in Oslo, and the cabin's
// forecast for the days someone is there.
func todayInput(cal calendar.Snapshot, hours []weather.Hourly, hytta map[string]weather.Daily, now time.Time) string {
	var b strings.Builder
	b.WriteString(nowLine(now))
	tomorrow := now.AddDate(0, 0, 1)
	b.WriteString("\nI dag, " + longDate(now) + ":\n")
	describeDay(&b, cal, now, now)
	b.WriteString("\nI morgen, " + longDate(tomorrow) + ":\n")
	describeDay(&b, cal, tomorrow, now)
	b.WriteString("\nVæret i Oslo, time for time:\n")
	for _, h := range hours[:min(len(hours), todayHours)] {
		item(&b, hourLine(h))
	}
	var cabin []string
	for _, d := range []time.Time{now, tomorrow} {
		if f, ok := hytta[dateKey(d)]; ok && atHytta(cal, d) {
			cabin = append(cabin, longDate(d)+": "+dayForecast(f))
		}
	}
	if len(cabin) > 0 {
		b.WriteString("\nVæret på hytta:\n")
		for _, c := range cabin {
			item(&b, c)
		}
	}
	return b.String()
}

// refreshToday writes a new sentence when the input has changed: about
// once an hour, and at once when the day turns.
func (c *Claude) refreshToday(ctx context.Context, cal calendar.Snapshot, now time.Time) {
	hours := c.src.Hourly(now)
	if len(hours) == 0 {
		return // no forecast yet
	}
	in := todayInput(cal, hours, c.src.Daily(weather.Hytta), now)
	hash, day := hashOf(in), dateKey(now)

	c.mu.Lock()
	last := c.today.written
	if hash == last.Hash {
		c.today.Checked = now
	}
	c.mu.Unlock()
	if !c.todayJob.due(last, hash, day, now) {
		return
	}

	var answer struct {
		Setning string `json:"setning"`
	}
	err := c.generate(ctx, request{
		name:   "today",
		system: todaySystem,
		user:   in,
		schema: object(map[string]any{"setning": str}),
		effort: anthropic.OutputConfigEffortMedium,
	}, &answer)
	var text string
	if err == nil {
		text, err = checkToday(answer.Setning, in)
	}
	if err != nil {
		c.todayJob.failed(now)
		slog.Warn("claude: no sentence for today", "err", err, "retry_in", c.todayJob.retry)
		return
	}
	c.todayJob.succeeded(now)
	c.mu.Lock()
	c.today = todayText{written{Hash: hash, Period: day, Checked: now}, text}
	c.dirty = true
	c.mu.Unlock()
	slog.Info("claude: today", "text", text)
}

// checkToday tidies the sentence and turns it down when it is too long or
// has a number that isn't in the input.
func checkToday(s, in string) (string, error) {
	s = tidy(s)
	if utf8.RuneCountInString(s) > todayMaxLen {
		return "", fmt.Errorf("too long: %q", s)
	}
	if !grounded(s, in) {
		return "", fmt.Errorf("a number that isn't in the input: %q", s)
	}
	return s, nil
}
