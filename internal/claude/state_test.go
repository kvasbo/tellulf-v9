package claude

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
)

func withState(c *Claude, dir, version string) *Claude {
	c.statePath, c.version = filepath.Join(dir, stateFile), version
	return c
}

// After a restart the saved texts show at once, and an unchanged input
// isn't asked for again.
func TestStateSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	long := timed("Invitasjon: Kvartalsgjennomgang økonomi og drift Q4 – ledergruppen", wed.Add(12*time.Hour), time.Hour)
	cal := calendar.Snapshot{Events: []calendar.Event{long}}
	f := &fake{answer: func(r request) (any, error) {
		switch r.name {
		case "today":
			return map[string]string{"setning": "Regn fra kl. 15"}, nil
		case "week":
			return map[string][]string{"linjer": {"Ingenting spesielt denne uka"}}, nil
		}
		return map[string]any{"hendelser": []map[string]string{{"id": "1", "tittel": "Kvartalsgjennomgang Q4", "merknad": ""}}}, nil
	}}
	c := withState(newTest(cal, f), dir, "v1")
	c.now = func() time.Time { return morning }
	c.refresh(context.Background())
	if len(f.requests) != 3 {
		t.Fatalf("%d requests, want 3", len(f.requests))
	}
	want := c.Texts(morning)

	later := morning.Add(20 * time.Minute)
	f2 := &fake{answer: f.answer}
	c2 := withState(newTest(cal, f2), dir, "v1")
	c2.load()
	got := c2.Texts(later)
	if got.Today != want.Today || !slices.Equal(got.Week, want.Week) || got.Titles.For(long) != want.Titles.For(long) {
		t.Fatalf("after a restart: %+v, want %+v", got, want)
	}
	if got.Today == "" || got.Titles.For(long).Short == "" {
		t.Fatalf("nothing was saved: %+v", got)
	}
	c2.now = func() time.Time { return later }
	c2.refresh(context.Background())
	if len(f2.requests) != 0 {
		t.Errorf("asked again after a restart: %d requests", len(f2.requests))
	}
}

// Texts saved with another model or other prompts are left behind, and so
// is a file that can't be read.
func TestStateFromElsewhereIsIgnored(t *testing.T) {
	dir := t.TempDir()
	f := &fake{answer: func(request) (any, error) { return map[string]string{"setning": "Regn fra kl. 15"}, nil }}
	c := withState(newTest(calendar.Snapshot{}, f), dir, "haiku")
	c.refreshToday(context.Background(), calendar.Snapshot{}, morning)
	c.save(morning)

	other := withState(newTest(calendar.Snapshot{}, f), dir, "opus")
	other.load()
	if got := other.Texts(morning).Today; got != "" {
		t.Errorf("loaded another model's sentence %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, stateFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := withState(newTest(calendar.Snapshot{}, f), dir, "haiku")
	broken.load()
	if got := broken.Texts(morning).Today; got != "" {
		t.Errorf("loaded %q from a broken file", got)
	}
	broken.save(morning)
	again := withState(newTest(calendar.Snapshot{}, f), dir, "haiku")
	again.load() // the broken file was overwritten; nothing in it, but readable
	if _, err := os.Stat(filepath.Join(dir, stateFile)); err != nil {
		t.Error(err)
	}
}

// The file is saved after a change, and otherwise every saveEvery so the
// time each input was last seen stays close.
func TestStateSavesOnChangeAndNowAndThen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, stateFile)
	f := &fake{answer: func(request) (any, error) { return map[string]string{"setning": ""}, nil }}
	c := withState(newTest(calendar.Snapshot{}, f), dir, "v1")
	now := morning
	c.now = func() time.Time { return now }

	c.refresh(context.Background())
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("not saved after the first texts: %v", err)
	}
	now = morning.Add(5 * time.Minute)
	c.refresh(context.Background())
	if b, _ := os.ReadFile(path); string(b) != string(first) {
		t.Error("saved again within saveEvery without a change")
	}
	now = morning.Add(saveEvery)
	c.refresh(context.Background())
	if b, _ := os.ReadFile(path); string(b) == string(first) {
		t.Error("not saved after saveEvery")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d files in the state directory, want 1", len(entries))
	}
}

// Before the calendar has loaded there are no events; the titles saved
// before a restart stay until it has.
func TestTitlesWaitForTheCalendar(t *testing.T) {
	long := timed("Invitasjon: Kvartalsgjennomgang økonomi og drift Q4 – ledergruppen", wed.Add(12*time.Hour), time.Hour)
	c := newTest(calendar.Snapshot{}, &fake{})
	c.titles = Titles{TitleKey(long): {Short: "Kvartalsgjennomgang Q4"}}
	c.refreshTitles(context.Background(), calendar.Snapshot{}, morning)
	if got := c.Texts(morning).Titles.For(long).Short; got != "Kvartalsgjennomgang Q4" {
		t.Errorf("title forgotten before the calendar loaded: %q", got)
	}
}
