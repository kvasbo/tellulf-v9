// Package claude has Claude write short texts for the screen from the
// calendar and the forecast: a sentence for today, short titles (and a note
// from the description) for events, and a few lines about the week ahead.
// The texts are written in the background when what they are written from
// changes, never while a page renders. Until a text is ready, once it has
// gone stale, or while Claude fails, there is none, and the screen looks as
// it does without Claude.
package claude

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// DefaultModel is the model used when the config doesn't name one.
const DefaultModel = "claude-haiku-5-5"

type Config struct {
	APIKey   string
	Model    string // DefaultModel when empty
	StateDir string // where the texts are kept between restarts; "" for memory only
}

// Sources are what the texts are written from.
type Sources struct {
	Calendar func() calendar.Snapshot
	Hourly   func(now time.Time) []weather.Hourly // Oslo, from the hour now
	Daily    func(loc weather.Location) map[string]weather.Daily
}

// Texts are what Claude has written that is still current. The zero value
// (no Claude, or nothing yet) leaves the screen as it is without Claude.
type Texts struct {
	Today  string   // a sentence for I DAG; "" when there is nothing worth saying
	Week   []string // the week ahead, one sentence each
	Titles Titles
}

type Claude struct {
	src      Sources
	generate generateFunc
	now      func() time.Time

	// The texts are replaced, never changed, so Texts can hand them out.
	mu     sync.RWMutex
	today  todayText
	week   weekText
	titles Titles

	// Only touched by Run's goroutine.
	todayJob, weekJob, titlesJob job
	dirty                        bool      // a text has changed since the last save
	saved                        time.Time // when the texts were last saved

	statePath string // "" without a state directory
	version   string // the model and the prompts the saved texts must match
}

// New makes a Claude that writes with the given API key.
func New(cfg Config, src Sources) *Claude {
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	slog.Info("Claude enabled", "model", model)
	c := newClaude(src, sdkGenerator(client, model))
	if cfg.StateDir != "" {
		c.statePath = filepath.Join(cfg.StateDir, stateFile)
		c.version = stateVersion(model)
		c.load()
	}
	return c
}

func newClaude(src Sources, generate generateFunc) *Claude {
	return &Claude{
		src:      src,
		generate: generate,
		now:      tz.Now,
		todayJob: job{interval: todayInterval},
		weekJob:  job{interval: weekInterval},
	}
}

// startDelay gives the calendar and the forecasts time to arrive, so the
// first texts aren't written from nothing.
const startDelay = time.Minute

// Run writes the texts as their sources change, until ctx is cancelled.
func (c *Claude) Run(ctx context.Context) {
	if !schedule.Sleep(ctx, startDelay) {
		return
	}
	schedule.Every(ctx, time.Minute, c.refresh)
}

func (c *Claude) refresh(ctx context.Context) {
	now := c.now().In(tz.Oslo)
	cal := c.src.Calendar()
	c.refreshTitles(ctx, cal, now)
	c.refreshToday(ctx, cal, now)
	c.refreshWeek(ctx, cal, now)
	if c.statePath != "" && (c.dirty || now.Sub(c.saved) >= saveEvery) {
		c.save(now)
	}
}

// Texts returns what is current at now: today's sentence on its day while
// its input has been seen lately, the week while it covers the same days,
// and the titles.
func (c *Claude) Texts(now time.Time) Texts {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t := Texts{Titles: c.titles}
	if c.today.current(dateKey(now), now, todayMaxAge) {
		t.Today = c.today.Text
	}
	if c.week.current(dateKey(WeekStart(now)), now, weekMaxAge) {
		t.Week = c.week.Lines
	}
	return t
}

// --- Pacing ------------------------------------------------------------------------

// written is what every text keeps about where it came from. The texts
// are saved as JSON (see state.go).
type written struct {
	Hash    string    `json:"hash"`    // of the input it was written from
	Period  string    `json:"period"`  // the date it is for: today, or the week's first day
	Checked time.Time `json:"checked"` // when that input was last seen unchanged
}

// current says whether the text is for period and still matches what it
// was written from: its input has been seen unchanged within maxAge.
func (w written) current(period string, now time.Time, maxAge time.Duration) bool {
	return w.Period == period && now.Sub(w.Checked) < maxAge
}

// maxRetry caps the wait after failed requests.
const maxRetry = 30 * time.Minute

// A job paces the requests for one kind of text: interval after a success;
// after a failure a minute, twice as long with each failure in a row, up to
// maxRetry.
type job struct {
	interval time.Duration
	next     time.Time     // no request before this
	retry    time.Duration // the wait after the last failure; 0 after a success
}

func (j *job) ready(now time.Time) bool { return !now.Before(j.next) }

func (j *job) succeeded(now time.Time) {
	j.retry = 0
	j.next = now.Add(j.interval)
}

func (j *job) failed(now time.Time) {
	j.retry = min(max(2*j.retry, time.Minute), maxRetry)
	j.next = now.Add(j.retry)
}

// due says whether to write a new text from the input hash for period,
// given the last one: when the input has changed and the job is ready, and
// at once when a new period has begun, unless requests are failing.
func (j *job) due(last written, hash, period string, now time.Time) bool {
	if hash == last.Hash {
		return false
	}
	if period != last.Period && j.retry == 0 {
		return true
	}
	return j.ready(now)
}

// --- Requests ----------------------------------------------------------------------

// A request asks Claude for one JSON answer.
type request struct {
	name   string // what it is for, in the log
	system string
	user   string
	schema map[string]any // the answer's JSON schema
	effort anthropic.OutputConfigEffort
}

// generateFunc sends r and decodes Claude's answer into out.
type generateFunc func(ctx context.Context, r request, out any) error

// requestTimeout bounds one request, the SDK's retries included.
const requestTimeout = 2 * time.Minute

func sdkGenerator(client anthropic.Client, model string) generateFunc {
	return func(ctx context.Context, r request, out any) error {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		start := time.Now()
		msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: 8000,
			System:    []anthropic.TextBlockParam{{Text: r.system}},
			Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(r.user))},
			OutputConfig: anthropic.OutputConfigParam{
				Effort: r.effort,
				Format: anthropic.JSONOutputFormatParam{Schema: r.schema},
			},
		})
		if err != nil {
			return err
		}
		slog.Debug("claude answered", "request", r.name, "model", msg.Model,
			"input_tokens", msg.Usage.InputTokens, "output_tokens", msg.Usage.OutputTokens,
			"took", time.Since(start).Round(time.Millisecond))
		switch msg.StopReason {
		case anthropic.StopReasonRefusal:
			return fmt.Errorf("declined (%s)", msg.StopDetails.Category)
		case anthropic.StopReasonMaxTokens:
			return errors.New("the answer was cut off")
		}
		for _, block := range msg.Content {
			if text, ok := block.AsAny().(anthropic.TextBlock); ok {
				return json.Unmarshal([]byte(text.Text), out)
			}
		}
		return errors.New("no text in the answer")
	}
}

// object is the JSON schema of an object whose properties are all required.
func object(props map[string]any) map[string]any {
	required := make([]string, 0, len(props))
	for name := range props {
		required = append(required, name)
	}
	slices.Sort(required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

func arrayOf(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

var str = map[string]any{"type": "string"}

// --- Checking answers --------------------------------------------------------------

var digits = regexp.MustCompile(`\d+`)

// numbers returns the numbers written in s, without leading zeros: "08:30"
// gives 8 and 30.
func numbers(s string) []string {
	var out []string
	for _, d := range digits.FindAllString(s, -1) {
		n, err := strconv.Atoi(d)
		if err != nil {
			continue // longer than an int: compare as written
		}
		d = strconv.Itoa(n)
		out = append(out, d)
	}
	return out
}

// grounded reports whether every number in text is also in source, so a
// text can't bring in a time, a date or a temperature of its own.
func grounded(text, source string) bool {
	have := map[string]bool{}
	for _, n := range numbers(source) {
		have[n] = true
	}
	for _, n := range numbers(text) {
		if !have[n] {
			return false
		}
	}
	return true
}

var minusBeforeDigit = regexp.MustCompile(`(^|[\s(])-(\d)`)

// tidy puts a text in the screen's style: no space or quotes around it, no
// full stop at the end, and a real minus sign before a number.
func tidy(s string) string {
	s = strings.TrimSpace(s)
	for _, q := range [][2]string{{`"`, `"`}, {"«", "»"}} {
		if len(s) < len(q[0])+len(q[1]) || !strings.HasPrefix(s, q[0]) || !strings.HasSuffix(s, q[1]) {
			continue
		}
		// Only one pair around it all: «Hytta» og «Taco» stays.
		if inner := s[len(q[0]) : len(s)-len(q[1])]; !strings.ContainsAny(inner, q[0]+q[1]) {
			s = strings.TrimSpace(inner)
		}
	}
	s = strings.TrimSpace(strings.TrimSuffix(s, "."))
	return minusBeforeDigit.ReplaceAllString(s, "${1}−${2}")
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// dateKey is t's date in Oslo: "2026-10-28".
func dateKey(t time.Time) string { return t.In(tz.Oslo).Format(time.DateOnly) }
