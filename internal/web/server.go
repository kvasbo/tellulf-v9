// Package web serves the page, the server-sent event stream and the static
// files.
package web

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/view"
	"github.com/kvasbo/tellulf-v9/internal/views"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

// Sources are the data sources the page is built from. Calendar, Smarthouse
// and Tibber are nil when not configured; their panels then render empty.
type Sources struct {
	Weather    *weather.Weather
	Entur      *entur.Entur
	Calendar   *calendar.Calendar
	Smarthouse *smarthouse.Smarthouse
	Tibber     *tibber.Tibber
}

type Server struct {
	src     Sources
	assets  fs.FS // contains static/ and public/
	version string
	hub     *Hub
}

// calendarDays is how many days the calendar column lays out; the client
// hides the ones that don't fit.
const calendarDays = 14

func New(src Sources, assets fs.FS) *Server {
	return &Server{
		src:    src,
		assets: assets,
		// A new version makes open browsers reload, so deploys show up.
		version: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		hub:     NewHub(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /sse", s.handleSSE)
	mux.HandleFunc("GET /", s.handleStatic)
	return mux
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
		{"sky", func(now time.Time) templ.Component { return views.Sky(s.sky(now)) }},
		{"current-weather", func(now time.Time) templ.Component { return views.CurrentWeather(s.currentWeather(now)) }},
		{"hourly-forecast", func(time.Time) templ.Component { return views.HourlyForecast(s.hourlyForecast()) }},
		{"calendar", func(now time.Time) templ.Component { return views.Calendar(s.calendar(now)) }},
		{"power-home", func(now time.Time) templ.Component { return views.Power(s.power(tibber.Home, now)) }},
		{"power-cabin", func(now time.Time) templ.Component { return views.Power(s.power(tibber.Cabin, now)) }},
		{"entur", func(time.Time) templ.Component { return views.Entur(s.trains()) }},
	}
	m := make(map[string]fragment, len(list))
	for _, f := range list {
		m[f.event] = f
	}
	return m
}

func (s *Server) page(now time.Time) views.Page {
	return views.Page{
		Version:        s.version,
		Sky:            s.sky(now),
		CurrentWeather: s.currentWeather(now),
		HourlyForecast: s.hourlyForecast(),
		Calendar:       s.calendar(now),
		PowerHome:      s.power(tibber.Home, now),
		PowerCabin:     s.power(tibber.Cabin, now),
		Trains:         s.trains(),
	}
}

func (s *Server) sky(now time.Time) view.Sky {
	return view.BuildSky(s.src.Weather.HourlyForecasts(weather.Oslo), now)
}

func (s *Server) currentWeather(now time.Time) view.CurrentWeather {
	return view.BuildCurrentWeather(s.readings(), now)
}

func (s *Server) hourlyForecast() view.HourlyForecast {
	return view.BuildHourlyForecast(s.src.Weather.HourlyForecasts(weather.Oslo))
}

func (s *Server) trains() []view.Train { return view.BuildTrains(s.src.Entur.Trains()) }

func (s *Server) readings() smarthouse.Readings {
	if s.src.Smarthouse == nil {
		return smarthouse.Readings{} // all nil: shown as dashes
	}
	return s.src.Smarthouse.Readings()
}

func (s *Server) calendar(now time.Time) view.Calendar {
	var snap calendar.Snapshot
	if s.src.Calendar != nil {
		snap = s.src.Calendar.Snapshot()
	}
	return view.BuildCalendar(snap,
		s.src.Weather.DailyForecasts(weather.Oslo),
		s.src.Weather.DailyForecasts(weather.Hytta),
		now, calendarDays)
}

func (s *Server) power(p tibber.Place, now time.Time) view.Power {
	var d tibber.PowerData
	if s.src.Tibber != nil {
		d = s.src.Tibber.PowerData(p)
	}
	return view.BuildPower(d, p, now)
}

// --- Handlers ----------------------------------------------------------------------

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	html, err := render(views.Layout(s.page(tz.Now())))
	if err != nil {
		slog.Error("render page", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func formatSSE(e Event) string {
	return "event: " + e.Name + "\ndata: " + strings.ReplaceAll(e.Data, "\n", "\ndata: ") + "\n\n"
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
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

	power := []string{"power-home", "power-cabin"}
	data := []string{"sky", "current-weather", "hourly-forecast", "calendar"}
	trains := []string{"entur"}

	publish(append(append(append([]string{}, data...), power...), trains...)...)

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
		case <-dataTick.C:
			publish(data...)
		case <-trainTick.C:
			publish(trains...)
		case <-versionTick.C:
			s.hub.Broadcast("version", s.version)
		}
	}
}
