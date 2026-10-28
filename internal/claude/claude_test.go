package claude

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Wednesday 28 October 2026, 07:00.
var (
	wed     = time.Date(2026, 10, 28, 0, 0, 0, 0, tz.Oslo)
	morning = wed.Add(7 * time.Hour)
)

func fullDay(title string, d time.Time) calendar.Event {
	return calendar.Event{Title: title, Start: d, End: d, FullDay: true}
}

func timed(title string, start time.Time, d time.Duration) calendar.Event {
	return calendar.Event{Title: title, Start: start, End: start.Add(d), Source: calendar.Audun}
}

func hours(from time.Time, n int) []weather.Hourly {
	var out []weather.Hourly
	for i := range n {
		t := from.Add(time.Duration(i) * time.Hour)
		temp := 4.0 + float64(i)/4
		h := weather.Hourly{Time: t, Hour: t.Hour(), Symbol: "cloudy", AirTemperature: &temp}
		if t.Hour() == 15 {
			h.Symbol, h.PrecipitationAmount = "rain", 0.8
		}
		out = append(out, h)
	}
	return out
}

// fake answers every request with what answer returns for it, as Claude's
// JSON, and counts the requests.
type fake struct {
	requests []request
	answer   func(r request) (any, error)
}

func (f *fake) generate(_ context.Context, r request, out any) error {
	f.requests = append(f.requests, r)
	v, err := f.answer(r)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(v)
	return json.Unmarshal(b, out)
}

func (f *fake) count(name string) int {
	n := 0
	for _, r := range f.requests {
		if r.name == name {
			n++
		}
	}
	return n
}

func newTest(cal calendar.Snapshot, f *fake) *Claude {
	return newClaude(Sources{
		Calendar: func() calendar.Snapshot { return cal },
		Hourly:   func(now time.Time) []weather.Hourly { return hours(now.Truncate(time.Hour), 30) },
		Daily: func(loc weather.Location) map[string]weather.Daily {
			return map[string]weather.Daily{
				"2026-10-28": {MinTemp: -3, MaxTemp: 2, LightRainProbability: 10},
				"2026-10-29": {MinTemp: -8, MaxTemp: 0, LightRainProbability: 0},
			}
		},
	}, f.generate)
}

func TestGroundedNumbers(t *testing.T) {
	source := "kl. 08: 4°\n- 16:30–17:00 Fotball\n- Hytta: −8° til 0°"
	for text, want := range map[string]bool{
		"Fotball kl. 16:30":            true,
		"Kaldt på hytta, −8°":          true,
		"Regn fra kl. 8":               true, // "08" in the input
		"Regn fra kl. 15":              false,
		"Ingen tall her":               true,
		"Fotball kl. 16:30, hjem 18":   false,
		"Kai fyller 45 på fredag":      false,
		"Tre avtaler før kl. 17 i dag": true,
	} {
		if got := grounded(text, source); got != want {
			t.Errorf("grounded(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestTidy(t *testing.T) {
	for in, want := range map[string]string{
		"  «Regn fra kl. 15.»  ": "Regn fra kl. 15",
		`"Kaldt på hytta, -8°"`:  "Kaldt på hytta, −8°",
		"Mellom -3 og 2°":        "Mellom −3 og 2°",
		"Bygg-2 starter":         "Bygg-2 starter",
		"Husk «Til hytta»":       "Husk «Til hytta»",
		"«Hytta» og «Taco»":      "«Hytta» og «Taco»",
		"":                       "",
	} {
		if got := tidy(in); got != want {
			t.Errorf("tidy(%q) = %q, want %q", in, got, want)
		}
	}
}

// A job waits its interval after a success, backs off after failures, and
// lets a new period through at once unless it is failing.
func TestJobPacing(t *testing.T) {
	j := job{interval: time.Hour}
	now := morning
	last := written{Hash: "a", Period: "2026-10-28"}
	if j.due(last, "a", "2026-10-28", now) {
		t.Error("due with the same input")
	}
	if !j.due(last, "b", "2026-10-28", now) {
		t.Error("not due with a new input")
	}
	j.succeeded(now)
	if j.due(last, "b", "2026-10-28", now.Add(59*time.Minute)) {
		t.Error("due within the interval")
	}
	if !j.due(last, "b", "2026-10-29", now.Add(time.Minute)) {
		t.Error("a new day waits for the interval")
	}
	j.failed(now)
	j.failed(now)
	if j.retry != 2*time.Minute {
		t.Errorf("retry after two failures = %v, want 2m", j.retry)
	}
	if j.due(last, "b", "2026-10-29", now.Add(time.Minute)) {
		t.Error("a new day skips the backoff")
	}
	for range 10 {
		j.failed(now)
	}
	if j.retry != maxRetry {
		t.Errorf("retry = %v, want the cap %v", j.retry, maxRetry)
	}
}

func TestTodayInput(t *testing.T) {
	thu := wed.AddDate(0, 0, 1)
	tannlege := timed("Tannlege", wed.Add(9*time.Hour), time.Hour)
	tannlege.Location = "Storgata 1, 0155 Oslo"
	cal := calendar.Snapshot{
		Events: []calendar.Event{
			timed("Frokostmøte", wed.Add(6*time.Hour), 30*time.Minute), // over at 07:00
			tannlege,
			{Title: "Foreldremøte", Start: wed.Add(18 * time.Hour), End: wed.Add(19 * time.Hour), Source: calendar.Felles}, // faded: the kids leave at 16
			timed("Til hytta", thu.Add(17*time.Hour), time.Hour),
		},
		Dinners:   []calendar.Event{fullDay("Taco", wed)},
		Birthdays: []calendar.Event{fullDay("Kai Lützenkirchen 1981", thu)},
		Barneuker: []calendar.Event{fullDay("Audun - Hanne", wed)},
	}
	c := newTest(cal, &fake{})
	in := todayInput(cal, c.src.Hourly(morning), c.src.Daily(weather.Hytta), morning)
	for _, want := range []string{
		"Nå: onsdag 28. oktober 2026, kl. 07\n",
		"I dag, onsdag 28. oktober:\n- Barna: drar til Hanne kl. 16\n- Middag: Taco\n- 09:00–10:00 Tannlege (Storgata 1)\n",
		"I morgen, torsdag 29. oktober:\n- Barna: hos Hanne\n- Middag: ingen planlagt\n- Bursdag: Kai Lützenkirchen fyller 45\n- 17:00–18:00 Til hytta\n",
		"- kl. 07: 4°, cloudy\n",
		"- kl. 15: 6°, rain, 0,8 mm nedbør\n",
		"Været på hytta:\n- torsdag 29. oktober: −8° til 0°, 0 % sjanse for regn\n",
	} {
		if !strings.Contains(in, want) {
			t.Errorf("input lacks %q:\n%s", want, in)
		}
	}
	for _, not := range []string{"Frokostmøte", "Foreldremøte", "onsdag 28. oktober: −3°"} {
		if strings.Contains(in, not) {
			t.Errorf("input has %q:\n%s", not, in)
		}
	}
	if n := strings.Count(in, "- kl. "); n != todayHours {
		t.Errorf("%d hours in the input, want %d", n, todayHours)
	}
}

// The sentence is written once per input, held for its day while the input
// is seen, and turned down when it brings in a number of its own.
func TestToday(t *testing.T) {
	sentence := "Regn fra kl. 15"
	f := &fake{answer: func(request) (any, error) { return map[string]string{"setning": sentence + "."}, nil }}
	c := newTest(calendar.Snapshot{}, f)
	ctx := context.Background()

	c.refreshToday(ctx, calendar.Snapshot{}, morning)
	c.refreshToday(ctx, calendar.Snapshot{}, morning.Add(30*time.Minute))
	if n := f.count("today"); n != 1 {
		t.Fatalf("%d requests in the same hour, want 1", n)
	}
	if got := c.Texts(morning.Add(30 * time.Minute)).Today; got != sentence {
		t.Errorf("today = %q, want %q", got, sentence)
	}
	if got := c.Texts(wed.AddDate(0, 0, 1).Add(time.Minute)).Today; got != "" {
		t.Errorf("tomorrow shows yesterday's sentence %q", got)
	}
	if got := c.Texts(morning.Add(30*time.Minute + todayMaxAge)).Today; got != "" {
		t.Errorf("a sentence whose input wasn't seen for %v still shows: %q", todayMaxAge, got)
	}

	// The next hour: a new input. This answer has a time that isn't in it.
	sentence = "Regn fra kl. 33"
	next := morning.Add(time.Hour)
	c.refreshToday(ctx, calendar.Snapshot{}, next)
	if n := f.count("today"); n != 2 {
		t.Fatalf("%d requests after an hour, want 2", n)
	}
	if got := c.Texts(next).Today; got != "Regn fra kl. 15" {
		t.Errorf("a turned-down sentence replaced the last one: %q", got)
	}
	if c.todayJob.retry != time.Minute {
		t.Errorf("retry = %v, want a minute after the first failure", c.todayJob.retry)
	}
}

func TestTodayWaitsForTheForecast(t *testing.T) {
	f := &fake{answer: func(request) (any, error) { return map[string]string{"setning": ""}, nil }}
	c := newTest(calendar.Snapshot{}, f)
	c.src.Hourly = func(time.Time) []weather.Hourly { return nil }
	c.refreshToday(context.Background(), calendar.Snapshot{}, morning)
	if len(f.requests) != 0 {
		t.Error("asked without a forecast")
	}
}

func TestWeekStart(t *testing.T) {
	for now, want := range map[time.Time]time.Time{
		wed.Add(17*time.Hour + 59*time.Minute): wed,
		wed.Add(18 * time.Hour):                wed.AddDate(0, 0, 1),
		wed.Add(23*time.Hour + 59*time.Minute): wed.AddDate(0, 0, 1),
	} {
		if got := WeekStart(now); !got.Equal(want) {
			t.Errorf("WeekStart(%v) = %v, want %v", now, got, want)
		}
	}
}

func TestWeekInput(t *testing.T) {
	cal := calendar.Snapshot{Events: []calendar.Event{timed("Middag på hytta", wed.Add(48*time.Hour+17*time.Hour), time.Hour)}}
	in := weekInput(cal, map[string]weather.Daily{"2026-10-28": {MinTemp: 2, MaxTemp: 6, LightRainProbability: 60}},
		map[string]weather.Daily{"2026-10-30": {MinTemp: -2, MaxTemp: 3}}, morning)
	for _, want := range []string{
		"Onsdag 28. oktober (i dag):\n- ingenting i kalenderen\n- Været i Oslo: 2° til 6°, 60 % sjanse for regn\n",
		"Torsdag 29. oktober (i morgen):\n",
		"Fredag 30. oktober:\n- 17:00–18:00 Middag på hytta\n- Været på hytta: −2° til 3°, 0 % sjanse for regn\n",
		"Tirsdag 3. november:\n",
	} {
		if !strings.Contains(in, want) {
			t.Errorf("input lacks %q:\n%s", want, in)
		}
	}
	if strings.Contains(in, "Onsdag 4. november") {
		t.Errorf("input has an eighth day:\n%s", in)
	}
}

func TestWeek(t *testing.T) {
	lines := []string{"Barna kommer på fredag kl. 16.", "Kaldt på fredag, ned mot −20°", "  "}
	f := &fake{answer: func(request) (any, error) { return map[string][]string{"linjer": lines}, nil }}
	cal := calendar.Snapshot{Barneuker: []calendar.Event{fullDay("Hanne - Audun", wed.AddDate(0, 0, 2))}}
	c := newTest(cal, f)
	ctx := context.Background()

	c.refreshWeek(ctx, cal, morning)
	c.refreshWeek(ctx, cal, morning.Add(4*time.Hour)) // the same input
	if n := f.count("week"); n != 1 {
		t.Fatalf("%d requests for the same input, want 1", n)
	}
	// −20° isn't in the input, so that line goes.
	if got, want := c.Texts(morning.Add(4*time.Hour)).Week, []string{"Barna kommer på fredag kl. 16"}; !slices.Equal(got, want) {
		t.Errorf("week = %q, want %q", got, want)
	}
	// From 18:00 the week starts tomorrow: the old one no longer applies.
	evening := wed.Add(18 * time.Hour)
	if got := c.Texts(evening).Week; got != nil {
		t.Errorf("the week from today shows in the evening: %q", got)
	}
	c.refreshWeek(ctx, cal, evening)
	if n := f.count("week"); n != 2 {
		t.Errorf("%d requests once the week starts tomorrow, want 2", n)
	}

	// Nothing worth saying is an answer.
	lines = nil
	c.weekJob = job{interval: weekInterval}
	c.refreshWeek(ctx, cal, evening.Add(23*time.Hour))
	if c.weekJob.retry != 0 || len(c.Texts(evening.Add(23*time.Hour)).Week) != 0 {
		t.Errorf("an empty answer: retry %v, week %q", c.weekJob.retry, c.Texts(evening.Add(23*time.Hour)).Week)
	}

	lines = []string{"Kaldt, ned mot −20°"}
	c.weekJob = job{interval: weekInterval}
	c.refreshWeek(ctx, cal, evening.Add(24*time.Hour))
	if got := c.Texts(evening.Add(24 * time.Hour)).Week; got != nil {
		t.Errorf("week without a usable line = %q", got)
	}
	if c.weekJob.retry == 0 {
		t.Error("no usable line is not a failure")
	}
}

func TestTitles(t *testing.T) {
	long := timed("Invitasjon: Foreldremøte for 3. trinn ved Slemdal skole @ ons. 4. nov.", wed.Add(18*time.Hour), time.Hour)
	described := timed("Fotball", wed.Add(17*time.Hour), time.Hour)
	described.Description = "<p>Hei!<br>Husk drikkeflaske og leggskinn.</p><a href=\"https://x\">Påmelding</a>"
	plain := timed("Tannlege", wed.Add(9*time.Hour), time.Hour)
	again := described
	again.Start = described.Start.AddDate(0, 0, 7) // the same, next week
	cal := calendar.Snapshot{Events: []calendar.Event{long, described, plain, again}}

	f := &fake{answer: func(r request) (any, error) {
		var items []struct{ ID, Tittel, Beskrivelse string }
		if err := json.Unmarshal([]byte(r.user), &items); err != nil {
			return nil, err
		}
		type entry struct {
			ID      string `json:"id"`
			Tittel  string `json:"tittel"`
			Merknad string `json:"merknad"`
		}
		var out []entry
		for _, it := range items {
			switch it.Tittel {
			case long.Title:
				out = append(out, entry{it.ID, "Foreldremøte 3. trinn", ""})
			case described.Title:
				if it.Beskrivelse != "Hei!\nHusk drikkeflaske og leggskinn.\nPåmelding" {
					t.Errorf("description sent as %q", it.Beskrivelse)
				}
				out = append(out, entry{it.ID, "Fotballtrening", "Drikkeflaske og leggskinn."})
			}
		}
		return map[string]any{"hendelser": out}, nil
	}}
	c := newTest(cal, f)
	ctx := context.Background()
	c.refreshTitles(ctx, cal, morning)
	c.refreshTitles(ctx, cal, morning.Add(time.Minute))
	if n := f.count("titles"); n != 1 {
		t.Fatalf("%d requests, want 1: every event is asked for once", n)
	}
	if n := strings.Count(f.requests[0].user, `"id"`); n != 2 {
		t.Errorf("%d events sent, want 2 (no plain titles, no repeats)", n)
	}

	titles := c.Texts(morning).Titles
	if got := titles.For(long); got != (Title{Short: "Foreldremøte 3. trinn"}) {
		t.Errorf("long title: %+v", got)
	}
	// A short title stays as it is, whatever Claude says; the note stays.
	if got := titles.For(again); got != (Title{Note: "Drikkeflaske og leggskinn"}) {
		t.Errorf("described event: %+v", got)
	}
	if got := titles.For(plain); got != (Title{}) {
		t.Errorf("plain event: %+v", got)
	}

	// Gone from the calendar, gone from the titles.
	c.refreshTitles(ctx, calendar.Snapshot{Events: []calendar.Event{plain}}, morning.Add(2*time.Minute))
	if n := len(c.Texts(morning).Titles); n != 0 {
		t.Errorf("%d titles kept for events that are gone", n)
	}
}

// A failed request is retried after the backoff; a skipped event isn't
// asked for again.
func TestTitlesFailAndSkip(t *testing.T) {
	long := timed("Et veldig langt møte om noe som tar mye plass på skjermen", wed.Add(18*time.Hour), time.Hour)
	cal := calendar.Snapshot{Events: []calendar.Event{long}}
	fail := true
	f := &fake{answer: func(request) (any, error) {
		if fail {
			return nil, errors.New("overloaded")
		}
		return map[string]any{"hendelser": []any{}}, nil
	}}
	c := newTest(cal, f)
	ctx := context.Background()
	c.refreshTitles(ctx, cal, morning)
	c.refreshTitles(ctx, cal, morning.Add(30*time.Second))
	if n := f.count("titles"); n != 1 {
		t.Fatalf("%d requests within the backoff, want 1", n)
	}
	fail = false
	c.refreshTitles(ctx, cal, morning.Add(time.Minute))
	c.refreshTitles(ctx, cal, morning.Add(2*time.Minute))
	if n := f.count("titles"); n != 2 {
		t.Errorf("%d requests, want 2: a skipped event isn't asked for again", n)
	}
	if got := c.Texts(morning).Titles.For(long); got != (Title{}) {
		t.Errorf("skipped event: %+v", got)
	}
}

func TestCheckTitle(t *testing.T) {
	e := calendar.Event{Title: "Invitasjon: Kvartalsmøte økonomi Q4 med ledergruppen", Description: "Oppmøte 08:45 i resepsjonen. Teams: https://teams.example/123456"}
	for _, c := range []struct {
		short, note string
		want        Title
	}{
		{"Kvartalsmøte Q4", "Oppmøte 08:45", Title{"Kvartalsmøte Q4", "Oppmøte 08:45"}},
		{"Kvartalsmøte Q4 kl. 9", "", Title{}},                                  // a number of its own
		{"Kvartalsmøte økonomi Q4 med hele ledergruppen i 4. etg", "", Title{}}, // too long
		{e.Title, "", Title{}}, // unchanged
		{"Kvartalsmøte", "Kvartalsmøte", Title{Short: "Kvartalsmøte"}}, // the note says nothing new
	} {
		if got := checkTitle(e, c.short, c.note); got != c.want {
			t.Errorf("checkTitle(%q, %q) = %+v, want %+v", c.short, c.note, got, c.want)
		}
	}
}

func TestPlainText(t *testing.T) {
	in := "<b>Velkommen</b> til &laquo;dugnad&raquo;!<br/><br>  Ta med   rive.<ul><li>Kaffe</li><li>Kake</li></ul>"
	if got, want := plainText(in), "Velkommen til «dugnad»!\nTa med rive.\nKaffe\nKake"; got != want {
		t.Errorf("plainText = %q, want %q", got, want)
	}
	if got := []rune(plainText(strings.Repeat("a", 2*descriptionMaxLen))); len(got) != descriptionMaxLen+1 {
		t.Errorf("a long description is %d characters, want %d", len(got), descriptionMaxLen+1)
	}
}
