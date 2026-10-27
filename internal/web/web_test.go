package web

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/mode"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

func TestHubSendsOnlyChanges(t *testing.T) {
	h := NewHub()
	h.Publish("a", "1")
	snapshot, updates, cancel := h.Subscribe()
	defer cancel()
	if len(snapshot) != 1 || snapshot[0] != (Event{"a", "1"}) {
		t.Fatalf("snapshot = %+v", snapshot)
	}

	h.Publish("a", "1") // unchanged: not sent
	h.Publish("b", "2")
	h.Publish("a", "3")
	h.Broadcast("version", "v")
	want := []Event{{"b", "2"}, {"a", "3"}, {"version", "v"}}
	for _, w := range want {
		if got := <-updates; got != w {
			t.Errorf("got %+v, want %+v", got, w)
		}
	}
	select {
	case e := <-updates:
		t.Errorf("unexpected extra event %+v", e)
	default:
	}
}

func TestHubDropsSlowSubscribers(t *testing.T) {
	h := NewHub()
	_, updates, cancel := h.Subscribe()
	defer cancel()
	for i := range 100 { // more than the buffer holds
		h.Broadcast("x", strings.Repeat("y", i))
	}
	n := 0
	for range updates { // the channel gets closed once the buffer overflows
		n++
	}
	if n == 0 || n >= 100 {
		t.Errorf("received %d events before being dropped", n)
	}
}

func TestFormatSSEPrefixesEveryLine(t *testing.T) {
	got := formatSSE(Event{"calendar", "<a>\n<b>"})
	want := "event: calendar\ndata: <a>\ndata: <b>\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// newTestServer serves the real static files from the repo,
// with data sources that have no data yet.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return New(Sources{Weather: weather.New(), Entur: entur.New()}, os.DirFS("../.."), mode.DefaultTimeout)
}

func TestPageAndStaticFiles(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Handler())
	defer srv.Close()

	cases := []struct {
		path, contains string
		status         int
	}{
		{"/", `data-sse="calendar"`, 200},
		{"/", `data-sse="weather"`, 200},
		{"/", "Inter+Tight", 200},
		{"/", `class="page-oversikt"`, 200},
		{"/strom", `data-mode-timeout="120"`, 200},
		{"/strom", `data-sse="strom-home"`, 200},
		{"/strom", `data-sse="strom-cabin"`, 200},
		{"/strom", `id="now_time"`, 200},
		{"/kalender", `data-sse="kalender"`, 200},
		{"/kalender", `class="page-kalender"`, 200},
		{"/vaer", `data-sse="vaer"`, 200},
		{"/strom/", "", 404},
		{"/favicon.ico", "", 200},
		{"/styles.css", "", 200},
		{"/vendor/idiomorph.esm.js", "", 404},
		{"/live.js", "EventSource", 200},
		{"/kids.svg", "", 404},
		{"/nope.txt", "", 404},
		{"/../go.mod", "", 404},
	}
	for _, c := range cases {
		res, err := http.Get(srv.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != c.status || !strings.Contains(string(body), c.contains) {
			t.Errorf("GET %s: %d, contains %q = %v", c.path, res.StatusCode, c.contains, strings.Contains(string(body), c.contains))
		}
	}
}

// Browsers must revalidate everything on every load: the files change with
// each deploy, and in dev mode under the browser's feet. Without this a
// reload keeps showing old CSS and JS (heuristic freshness from Last-Modified).
func TestNothingIsCached(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Handler())
	defer srv.Close()
	for _, p := range []string{"/", "/styles.css", "/client.js", "/favicon.ico"} {
		res, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if got := res.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("GET %s: Cache-Control = %q, want no-cache", p, got)
		}
	}
}

func TestClientLog(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Handler())
	defer srv.Close()
	res, err := http.Post(srv.URL+"/log", "text/plain", strings.NewReader("sky: context lost"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("POST /log: %d", res.StatusCode)
	}
}

func TestSSEStartsWithSnapshot(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Publish(ctx)

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	waitForFragments(t, s, 9)

	names, data := readEvents(t, srv.URL+"/sse", 10)
	// The version follows the snapshot, so a browser reconnecting after a
	// restart can reload at once.
	want := "weather calendar kalender vaer power-home power-cabin strom-home strom-cabin trains version"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
	if data["version"] != s.version {
		t.Errorf("version = %q, want %q", data["version"], s.version)
	}
}

// A page asks for the events it shows, and gets only those: the overview
// doesn't need the power page's fragments every second.
func TestSSESendsOnlyTheEventsAskedFor(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Publish(ctx)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	waitForFragments(t, s, 9)

	names, _ := readEvents(t, srv.URL+"/sse?events=strom-cabin,kalender,version,mode", 3)
	if got, want := strings.Join(names, " "), "kalender strom-cabin version"; got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
}

// waitForFragments gives the publisher a moment to render its first round.
func waitForFragments(t *testing.T, s *Server, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		snap, _, c := s.hub.Subscribe()
		c()
		if len(snap) == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("publisher rendered %d fragments, want %d", len(snap), n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// readEvents reads n whole events (name and data) from an event stream.
func readEvents(t *testing.T, url string, n int) (names []string, data map[string]string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content type %q", ct)
	}
	data = map[string]string{}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var name string
	for len(names) < n && sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data[name] += strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			names = append(names, name)
			name = ""
		}
	}
	return names, data
}

// The server owns the mode and gives it to open pages over SSE, also in the
// snapshot a reconnecting page starts from. It never redirects: any page
// can be opened directly, whatever the kiosk shows.
func TestModeIsInTheSnapshotAndNeverRedirects(t *testing.T) {
	s := newTestServer(t)
	var current atomic.Value // mode.Mode; the publisher reads it too
	current.Store(mode.Mode(""))
	s.mode = func() mode.Mode { return current.Load().(mode.Mode) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Publish(ctx)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(path string) (int, string) {
		res, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode, res.Header.Get("Location")
	}

	pages := []string{"/", "/strom", "/kalender", "/vaer"}
	for _, m := range []mode.Mode{"", mode.Oversikt, mode.Strom} {
		current.Store(m)
		for _, p := range pages {
			if code, loc := get(p); code != http.StatusOK {
				t.Errorf("GET %s in mode %q: %d to %q, want 200", p, m, code, loc)
			}
		}
	}

	// The publisher picks the mode up within a tick.
	deadline := time.Now().Add(3 * time.Second)
	for {
		snap, _, c := s.hub.Subscribe()
		c()
		if slices.Contains(snap, Event{"mode", "/strom"}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the mode never reached the hub")
		}
		time.Sleep(20 * time.Millisecond)
	}
	names, data := readEvents(t, srv.URL+"/sse?events=mode,version", 2)
	if strings.Join(names, " ") != "mode version" || data["mode"] != "/strom" {
		t.Errorf("events %q, mode %q", names, data["mode"])
	}
}
