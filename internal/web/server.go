// Package web serves the page, the server-sent event stream and the static
// files.
package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/claude"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/mode"
	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/view"
	"github.com/kvasbo/tellulf-v9/internal/views"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Sources are the data sources the pages are built from. Calendar,
// Smarthouse, Tibber and Claude are nil when not configured; their panels
// then render empty (without Claude: as they were before it), and without
// Smarthouse there is no mode to follow.
type Sources struct {
	Weather    *weather.Weather
	Entur      *entur.Entur
	Calendar   *calendar.Calendar
	Smarthouse *smarthouse.Smarthouse
	Tibber     *tibber.Tibber
	Claude     *claude.Claude
}

type Server struct {
	src     Sources
	assets  fs.FS // contains static/ and public/
	started time.Time
	version string
	hub     *Hub
	// mode is the page the screens should show, or "" without MQTT: the
	// one MQTT last asked for, until it has been up for modeTimeout (0: for
	// good). The server owns it and pushes it to open pages over SSE.
	mode        func() mode.Mode
	modeTimeout time.Duration
}

// calendarDays is how many days the calendar column and the calendar page
// lay out; the ones that don't fit are hidden.
const calendarDays = 14

// New makes a server for src. modeTimeout is how long a page other than
// the overview stays up, both one MQTT picked and one opened directly; 0
// keeps it for good.
func New(src Sources, assets fs.FS, modeTimeout time.Duration) *Server {
	started := time.Now()
	return &Server{
		src:     src,
		assets:  assets,
		started: started,
		// A new version makes open browsers reload, so deploys show up.
		version: started.UTC().Format("2006-01-02T15:04:05.000Z"),
		hub:     NewHub(),
		mode: func() mode.Mode {
			if src.Smarthouse == nil {
				return ""
			}
			m, since := src.Smarthouse.Mode()
			return mode.Current(m, since, time.Now(), modeTimeout)
		},
		modeTimeout: modeTimeout,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage(func(now time.Time) templ.Component {
		return views.Layout(s.page(now))
	}))
	mux.HandleFunc("GET "+mode.Strom.Path(), s.handlePage(func(now time.Time) templ.Component {
		return views.StromLayout(s.stromPage(now))
	}))
	mux.HandleFunc("GET "+mode.Kalender.Path(), s.handlePage(func(now time.Time) templ.Component {
		return views.KalenderLayout(s.kalenderPage(now))
	}))
	mux.HandleFunc("GET "+mode.Vaer.Path(), s.handlePage(func(now time.Time) templ.Component {
		return views.VaerLayout(s.vaerPage(now))
	}))
	mux.HandleFunc("GET "+mode.Uke.Path(), s.handlePage(func(now time.Time) templ.Component {
		return views.UkeLayout(s.ukePage(now))
	}))
	mux.HandleFunc("GET /sse", s.handleSSE)
	mux.HandleFunc("POST /log", handleLog)
	mux.HandleFunc("GET /", s.handleStatic)
	// Nothing here is worth caching: the page and the static files change
	// with every deploy, and in dev mode under the browser's feet. no-cache
	// makes browsers revalidate on every load instead of trusting the
	// heuristic freshness they derive from Last-Modified.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		mux.ServeHTTP(w, r)
	})
}

// --- Rendering ---------------------------------------------------------------------

func render(c templ.Component) (string, error) {
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// A fragment is one part of the page that is pushed over SSE on its own.
type fragment struct {
	event     string
	component func(now time.Time) templ.Component
}

func (s *Server) fragments() map[string]fragment {
	list := []fragment{
		{"weather", func(now time.Time) templ.Component { return views.Weather(s.weather(now)) }},
		{"calendar", func(now time.Time) templ.Component {
			return views.Calendar(s.calendar(now), view.BuildSun(now), view.BuildStamps(s.started))
		}},
		{"power-home", func(now time.Time) templ.Component { return views.Power(s.power(tibber.Home)) }},
		{"power-cabin", func(now time.Time) templ.Component { return views.Power(s.power(tibber.Cabin)) }},
		{"trains", func(now time.Time) templ.Component { return views.Entur(s.trains(now)) }},
		{"strom-home", func(now time.Time) templ.Component { return views.Strom(s.strom(tibber.Home, now)) }},
		{"strom-cabin", func(now time.Time) templ.Component { return views.Strom(s.strom(tibber.Cabin, now)) }},
		{"kalender", func(now time.Time) templ.Component { return views.Agenda(s.agenda(now)) }},
		{"vaer", func(now time.Time) templ.Component { return views.Vaer(s.vaer(now)) }},
		{"uke", func(now time.Time) templ.Component { return views.Uke(s.uke(now)) }},
	}
	m := make(map[string]fragment, len(list))
	for _, f := range list {
		m[f.event] = f
	}
	return m
}

// frame is what every page has besides its content.
func (s *Server) frame(now time.Time) views.Frame {
	return views.Frame{
		Version:     s.version,
		ModeTimeout: strconv.Itoa(int(s.modeTimeout.Seconds())),
		Clock:       view.BuildClock(now),
		Stamps:      view.BuildStamps(s.started),
	}
}

func (s *Server) page(now time.Time) views.Page {
	return views.Page{
		Frame:      s.frame(now),
		Weather:    s.weather(now),
		Sun:        view.BuildSun(now),
		Calendar:   s.calendar(now),
		PowerHome:  s.power(tibber.Home),
		PowerCabin: s.power(tibber.Cabin),
		Trains:     s.trains(now),
	}
}

func (s *Server) stromPage(now time.Time) views.StromPage {
	return views.StromPage{
		Frame: s.frame(now),
		Home:  s.strom(tibber.Home, now),
		Cabin: s.strom(tibber.Cabin, now),
	}
}

func (s *Server) kalenderPage(now time.Time) views.KalenderPage {
	return views.KalenderPage{
		Frame:  s.frame(now),
		Agenda: s.agenda(now),
	}
}

func (s *Server) vaerPage(now time.Time) views.VaerPage {
	return views.VaerPage{
		Frame: s.frame(now),
		Vaer:  s.vaer(now),
	}
}

func (s *Server) ukePage(now time.Time) views.UkePage {
	return views.UkePage{
		Frame: s.frame(now),
		Uke:   s.uke(now),
	}
}

func (s *Server) uke(now time.Time) view.Uke {
	return view.BuildUke(s.calendarSnapshot(), s.src.Weather.DailyForecasts(weather.Oslo), s.texts(now), now)
}

func (s *Server) vaer(now time.Time) view.Vaer {
	return view.BuildVaer(s.readings(), s.src.Weather.HourlyForecasts(weather.Oslo, now),
		s.src.Weather.DailyForecasts(weather.Oslo), s.src.Weather.DailyForecasts(weather.Hytta), now)
}

func (s *Server) weather(now time.Time) view.Weather {
	return view.BuildWeather(s.readings(), s.src.Weather.HourlyForecasts(weather.Oslo, now), now)
}

func (s *Server) trains(now time.Time) view.Trains {
	return view.BuildTrains(s.src.Entur.Trains(), now)
}

func (s *Server) readings() smarthouse.Readings {
	if s.src.Smarthouse == nil {
		return smarthouse.Readings{} // all nil: the forecast stands in
	}
	return s.src.Smarthouse.Readings()
}

func (s *Server) calendar(now time.Time) view.Calendar {
	return view.BuildCalendar(s.calendarSnapshot(),
		s.src.Weather.DailyForecasts(weather.Oslo),
		s.src.Weather.DailyForecasts(weather.Hytta),
		s.texts(now), now, calendarDays)
}

func (s *Server) calendarSnapshot() calendar.Snapshot {
	if s.src.Calendar == nil {
		return calendar.Snapshot{}
	}
	return s.src.Calendar.Snapshot()
}

func (s *Server) agenda(now time.Time) view.Agenda {
	return view.BuildAgenda(s.calendarSnapshot(),
		s.src.Weather.DailyForecasts(weather.Oslo),
		s.src.Weather.DailyForecasts(weather.Hytta),
		s.texts(now).Titles, now, calendarDays)
}

// texts is what Claude has written; nothing without Claude.
func (s *Server) texts(now time.Time) claude.Texts {
	if s.src.Claude == nil {
		return claude.Texts{}
	}
	return s.src.Claude.Texts(now)
}

func (s *Server) strom(p tibber.Place, now time.Time) view.Strom {
	var d tibber.Details
	if s.src.Tibber != nil {
		d = s.src.Tibber.Details(p)
	}
	return view.BuildStrom(d, p, now)
}

func (s *Server) power(p tibber.Place) view.Power {
	var d tibber.PowerData
	if s.src.Tibber != nil {
		d = s.src.Tibber.PowerData(p)
	}
	return view.BuildPower(d, p)
}

// --- Handlers ----------------------------------------------------------------------

// handlePage serves a page, rendered with the latest data. Every page opens
// where it is asked for; client.js follows the mode from there.
func (s *Server) handlePage(page func(now time.Time) templ.Component) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		html, err := render(page(tz.Now()))
		if err != nil {
			slog.Error("render page", "path", r.URL.Path, "err", err)
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	}
}

func formatSSE(e Event) string {
	return "event: " + e.Name + "\ndata: " + strings.ReplaceAll(e.Data, "\n", "\ndata: ") + "\n\n"
}

// handleSSE streams events: the latest of each fragment first, then every
// change. ?events=a,b,c limits the stream to those names, so a page only
// gets what it shows; without it, everything is sent.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	var only map[string]bool
	if list := r.URL.Query().Get("events"); list != "" {
		only = map[string]bool{}
		for name := range strings.SplitSeq(list, ",") {
			only[name] = true
		}
	}

	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	snapshot, updates, cancel := s.hub.Subscribe()
	defer cancel()

	send := func(e Event) bool {
		if only != nil && !only[e.Name] {
			return true
		}
		if _, err := fmt.Fprint(w, formatSSE(e)); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	// Send the version right away too, so a browser that reconnects after a
	// restart reloads immediately instead of waiting for the minute tick.
	for _, e := range append(snapshot, Event{"version", s.version}) {
		if !send(e) {
			return
		}
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-updates:
			if !ok || !send(e) {
				return
			}
		}
	}
}

// handleLog writes a line from the browser to the server log.
func handleLog(w http.ResponseWriter, r *http.Request) {
	msg, err := io.ReadAll(io.LimitReader(r.Body, 1024))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	slog.Info("client", "msg", string(msg))
	w.WriteHeader(http.StatusNoContent)
}

// handleStatic serves files from static/, then public/.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	for _, dir := range []string{"static", "public"} {
		p := path.Join(dir, name)
		if info, err := fs.Stat(s.assets, p); err == nil && !info.IsDir() {
			http.ServeFileFS(w, r, s.assets, p)
			return
		}
	}
	http.NotFound(w, r)
}

// --- Publishing --------------------------------------------------------------------

// Publish renders fragments on their schedules and pushes changes to every
// connected browser, until ctx is cancelled.
func (s *Server) Publish(ctx context.Context) {
	frag := s.fragments()
	publish := func(names ...string) {
		now := tz.Now()
		for _, name := range names {
			html, err := render(frag[name].component(now))
			if err != nil {
				slog.Error("render fragment", "fragment", name, "err", err)
				continue
			}
			s.hub.Publish(name, html)
		}
	}

	// The mode is published like a fragment, so the hub only sends it when
	// it changes (also when a page times out back to the overview) and new
	// connections get it in their snapshot. Without MQTT there is none, and
	// screens stay on the page they opened.
	publishMode := func() {
		if m := s.mode(); m != "" {
			s.hub.Publish("mode", m.Path())
		}
	}

	power := []string{"power-home", "power-cabin", "strom-home", "strom-cabin"}
	data := []string{"weather", "calendar", "kalender", "vaer", "uke"}
	trains := []string{"trains"}

	publish(append(append(append([]string{}, data...), power...), trains...)...)
	publishMode()

	powerTick := time.NewTicker(time.Second)
	dataTick := time.NewTicker(15 * time.Second)
	trainTick := time.NewTicker(30 * time.Second)
	versionTick := time.NewTicker(time.Minute)
	defer powerTick.Stop()
	defer dataTick.Stop()
	defer trainTick.Stop()
	defer versionTick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-powerTick.C:
			publish(power...)
			publishMode()
		case <-dataTick.C:
			publish(data...)
		case <-trainTick.C:
			publish(trains...)
		case <-versionTick.C:
			s.hub.Broadcast("version", s.version)
		}
	}
}
