//go:build live

package claude

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// TestLive writes the three texts from a made-up week with the real API,
// to judge the prompts and compare models. It costs a few øre:
//
//	ANTHROPIC_API_KEY=... go test -tags live -run Live -v ./internal/claude
//
// TELLULF_CLAUDE_MODEL picks the model; DefaultModel without it. LIVE_AT
// sets the time, "2026-10-28T07:00" by default. The log has the tokens
// each request used.
func TestLive(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}
	model := os.Getenv("TELLULF_CLAUDE_MODEL")
	if model == "" {
		model = DefaultModel
	}
	now := morning
	if at := os.Getenv("LIVE_AT"); at != "" {
		var err error
		if now, err = time.ParseInLocation("2006-01-02T15:04", at, wed.Location()); err != nil {
			t.Fatal(err)
		}
	}

	fri := wed.AddDate(0, 0, 2)
	teams := timed("Invitasjon: Kvartalsgjennomgang økonomi og drift Q4 – ledergruppen (Teams) @ ons. 28. okt. 2026 12:00 - 13:30", wed.Add(12*time.Hour), 90*time.Minute)
	teams.Description = `<p>Hei alle,</p><p>Vi går gjennom tallene for Q4. Les vedlagt rapport før møtet.</p>________________________________________________________________________________<br><b>Microsoft Teams</b><br>Bli med i møtet nå<br>Møte-ID: 384 221 907 55<br>Passord: x7Ab9c`
	fotball := calendar.Event{Title: "Fotballtrening Engebret", Start: wed.Add(17 * time.Hour), End: wed.Add(18 * time.Hour), Source: calendar.Felles,
		Description: "Oppmøte 16:45 ved kunstgressbanen. Husk drikkeflaske og leggskinn!"}
	cal := calendar.Snapshot{
		Events: []calendar.Event{
			teams,
			fotball,
			timed("Tannlege", wed.AddDate(0, 0, 1).Add(8*time.Hour), time.Hour),
			{Title: "Foreldremøte 3. trinn Slemdal skole, aula", Start: wed.AddDate(0, 0, 1).Add(18 * time.Hour), End: wed.AddDate(0, 0, 1).Add(19*time.Hour + 30*time.Minute), Source: calendar.Felles},
			timed("Til hytta", fri.Add(17*time.Hour), 2*time.Hour),
			fullDay("Hytta", fri.AddDate(0, 0, 1)),
			timed("Middag hos Kari og Per", wed.AddDate(0, 0, 5).Add(18*time.Hour), 3*time.Hour),
		},
		Dinners:   []calendar.Event{fullDay("Taco", wed), fullDay("Fiskeboller", wed.AddDate(0, 0, 1)), fullDay("Pizza", fri)},
		Birthdays: []calendar.Event{fullDay("Kai Lützenkirchen 1981", wed.AddDate(0, 0, 1))},
		Barneuker: []calendar.Event{fullDay("Audun", wed), fullDay("Audun", wed.AddDate(0, 0, 1)), fullDay("Audun", fri),
			fullDay("Audun", fri.AddDate(0, 0, 1)), fullDay("Audun", fri.AddDate(0, 0, 2)), fullDay("Audun - Hanne", fri.AddDate(0, 0, 3))},
	}
	oslo := map[string]weather.Daily{}
	hytta := map[string]weather.Daily{}
	for i := range 8 {
		d := dateKey(wed.AddDate(0, 0, i))
		oslo[d] = weather.Daily{MinTemp: float64(2 - i), MaxTemp: float64(8 - i), LightRainProbability: float64([]int{70, 30, 10, 0, 0, 20, 60, 50}[i])}
		hytta[d] = weather.Daily{MinTemp: float64(-1 - 2*i), MaxTemp: float64(5 - i), LightRainProbability: 10}
	}

	client := anthropic.NewClient(option.WithAPIKey(key))
	c := newClaude(Sources{
		Calendar: func() calendar.Snapshot { return cal },
		Hourly:   func(now time.Time) []weather.Hourly { return hours(now.Truncate(time.Hour), 30) },
		Daily: func(loc weather.Location) map[string]weather.Daily {
			if loc == weather.Hytta {
				return hytta
			}
			return oslo
		},
	}, sdkGenerator(client, model))
	c.now = func() time.Time { return now }

	start := time.Now()
	c.refresh(context.Background())
	texts := c.Texts(now)
	t.Logf("model %s, %v", model, time.Since(start).Round(time.Millisecond))
	t.Logf("today: %q", texts.Today)
	for _, l := range texts.Week {
		t.Logf("week:  %q", l)
	}
	for _, e := range cal.Events {
		if tt := texts.Titles.For(e); tt != (Title{}) {
			t.Logf("title: %q → %q, note %q", e.Title, tt.Short, tt.Note)
		}
	}
	if len(texts.Week) == 0 {
		t.Error("no week")
	}
}
