package web

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/entur"
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

// newTestServer runs the real templates and static files from the repo,
// with data sources that have no data yet.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Sources{Weather: weather.New(), Entur: entur.New()}, os.DirFS("../.."), false)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPageAndStaticFiles(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Handler())
	defer srv.Close()

	cases := []struct {
		path, contains string
		status         int
	}{
		{"/", `sse-connect="/sse"`, 200},
		{"/", `<div id="current_temperature">–</div>`, 200}, // no sensor data yet
		{"/styles.css", "", 200},
		{"/vendor/htmx.min.js", "htmx", 200},
		{"/kids.svg", "<svg", 200},
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

func TestSSEStartsWithSnapshot(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Publish(ctx)

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Give the publisher a moment to render the first round.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if snap, _, c := s.hub.Subscribe(); len(snap) == 7 {
			c()
			break
		} else {
			c()
		}
		if time.Now().After(deadline) {
			t.Fatal("publisher never rendered all fragments")
		}
		time.Sleep(10 * time.Millisecond)
	}

	res, err := http.Get(srv.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content type %q", ct)
	}
	// Read whole events (name + data) until the snapshot and version are in.
	var names []string
	data := map[string]string{}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var name string
	for len(names) < 8 && sc.Scan() {
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
	// The version follows the snapshot, so a browser reconnecting after a
	// restart can reload at once.
	want := "sky current-weather hourly-forecast calendar power-home power-cabin entur version"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
	if data["version"] != s.version {
		t.Errorf("version = %q, want %q", data["version"], s.version)
	}
}
