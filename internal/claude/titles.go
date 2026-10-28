package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"log/slog"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
)

const (
	// shortTitleFrom: titles longer than this, in characters, get a short
	// one for the overview.
	shortTitleFrom = 30
	// titleMaxLen and noteMaxLen bound what is accepted.
	titleMaxLen = 40
	noteMaxLen  = 60
	// titlesPerRequest bounds one request; the rest wait for the next minute.
	titlesPerRequest = 25
	// descriptionMaxLen bounds how much of a description is sent.
	descriptionMaxLen = 1500
)

const titlesSystem = `Du rydder kalenderhendelser for en hjemmeskjerm. Du får en liste med hendelser i JSON, hver med id, tittel og kanskje en beskrivelse. Svar med én oppføring per hendelse, med samme id:

- tittel: en kort tittel, helst under 30 tegn, på samme språk som originalen. Behold det viktigste: hva det er og hvem det gjelder. Dropp lenker, adresser, møteplattform, «Invitasjon:», «FW:» og gjentakelser. Ikke legg til noe som ikke står i originalen. Er tittelen allerede kort, gi den tilbake uendret.
- merknad: én kort ting fra beskrivelsen som er nyttig å vite på forhånd, som hva man skal ha med seg eller når man skal møte opp, høyst 40 tegn. Ellers tom. Aldri møtelenker, møte-ID, passord, telefonnumre, påmeldingsfrister eller noe som bare gjentar tittelen.`

// Title is what Claude made of an event: a short title for the overview,
// and a note from the description for the calendar page. Either may be "",
// and the screen then shows what it would without Claude.
type Title struct {
	Short string
	Note  string
}

// Titles are keyed by what an event says (TitleKey), so the events of a
// series share one.
type Titles map[string]Title

// For returns what Claude made of e; the zero Title if nothing.
func (t Titles) For(e calendar.Event) Title { return t[TitleKey(e)] }

// TitleKey is e's key in Titles: its title and description.
func TitleKey(e calendar.Event) string { return hashOf(e.Title + "\x00" + e.Description)[:16] }

// needsTitle: an event is sent to Claude when its title is long or it has
// a description.
func needsTitle(e calendar.Event) bool {
	return utf8.RuneCountInString(e.Title) > shortTitleFrom || plainText(e.Description) != ""
}

// refreshTitles asks for the events it hasn't seen, a batch at a time, and
// forgets the ones that have left the calendar.
func (c *Claude) refreshTitles(ctx context.Context, cal calendar.Snapshot, now time.Time) {
	c.mu.RLock()
	have := c.titles
	c.mu.RUnlock()

	seen := map[string]calendar.Event{}
	var todo []calendar.Event
	for _, e := range cal.Events {
		if !needsTitle(e) {
			continue
		}
		k := TitleKey(e)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = e
		if _, ok := have[k]; !ok {
			todo = append(todo, e)
		}
	}

	var got Titles
	if len(todo) > 0 && c.titlesJob.ready(now) {
		var err error
		got, err = c.writeTitles(ctx, todo[:min(len(todo), titlesPerRequest)])
		if err != nil {
			c.titlesJob.failed(now)
			slog.Warn("claude: no titles", "err", err, "retry_in", c.titlesJob.retry)
		} else {
			c.titlesJob.succeeded(now)
			for k, t := range got {
				if t != (Title{}) {
					slog.Info("claude: title", "title", seen[k].Title, "short", t.Short, "note", t.Note)
				}
			}
		}
	}

	next := make(Titles, len(seen))
	for k := range seen {
		if t, ok := got[k]; ok {
			next[k] = t
		} else if t, ok := have[k]; ok {
			next[k] = t
		}
	}
	if !maps.Equal(next, have) {
		c.mu.Lock()
		c.titles = next
		c.mu.Unlock()
	}
}

// writeTitles asks for events' short titles and notes. Every event gets an
// entry, also one Claude skipped or answered badly, so it isn't asked for
// again.
func (c *Claude) writeTitles(ctx context.Context, events []calendar.Event) (Titles, error) {
	type item struct {
		ID          string `json:"id"`
		Title       string `json:"tittel"`
		Description string `json:"beskrivelse,omitempty"`
	}
	items := make([]item, len(events))
	for i, e := range events {
		items[i] = item{ID: strconv.Itoa(i + 1), Title: e.Title, Description: plainText(e.Description)}
	}
	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return nil, err
	}

	var answer struct {
		Hendelser []struct {
			ID      string `json:"id"`
			Tittel  string `json:"tittel"`
			Merknad string `json:"merknad"`
		} `json:"hendelser"`
	}
	err := c.generate(ctx, request{
		name:   "titles",
		system: titlesSystem,
		user:   in.String(),
		schema: object(map[string]any{
			"hendelser": arrayOf(object(map[string]any{"id": str, "tittel": str, "merknad": str})),
		}),
		effort: anthropic.OutputConfigEffortLow,
	}, &answer)
	if err != nil {
		return nil, err
	}

	out := make(Titles, len(events))
	for _, e := range events {
		out[TitleKey(e)] = Title{}
	}
	for _, a := range answer.Hendelser {
		i, err := strconv.Atoi(a.ID)
		if err != nil || i < 1 || i > len(events) {
			continue
		}
		e := events[i-1]
		out[TitleKey(e)] = checkTitle(e, a.Tittel, a.Merknad)
	}
	return out, nil
}

// checkTitle keeps a short title only for a long title, and either part
// only when it is short enough and brings in no number of its own.
func checkTitle(e calendar.Event, short, note string) Title {
	source := e.Title + "\n" + plainText(e.Description)
	var t Title
	short = tidy(short)
	if utf8.RuneCountInString(e.Title) > shortTitleFrom && short != "" && short != e.Title &&
		utf8.RuneCountInString(short) <= titleMaxLen && grounded(short, source) {
		t.Short = short
	}
	note = tidy(note)
	if note != "" && utf8.RuneCountInString(note) <= noteMaxLen && grounded(note, source) &&
		!strings.EqualFold(note, e.Title) && !strings.EqualFold(note, t.Short) {
		t.Note = note
	}
	return t
}

var (
	htmlBreak = regexp.MustCompile(`(?i)<br\s*/?>|</?(p|div|li|ul|ol|h\d)\b[^>]*>`)
	htmlTag   = regexp.MustCompile(`<[^>]*>`)
	spaces    = regexp.MustCompile(`\s+`)
)

// plainText turns a description, often HTML, into plain lines, at most
// descriptionMaxLen characters of them.
func plainText(s string) string {
	s = htmlBreak.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(spaces.ReplaceAllString(line, " ")); line != "" {
			lines = append(lines, line)
		}
	}
	s = strings.Join(lines, "\n")
	if r := []rune(s); len(r) > descriptionMaxLen {
		s = string(r[:descriptionMaxLen]) + "…"
	}
	return s
}
