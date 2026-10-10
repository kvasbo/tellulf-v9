// Tellulf: a home status display showing weather, calendars, metro
// departures and power usage, rendered on the server and kept live over SSE.
package main

import (
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/entur"
	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/tibber"
	"github.com/kvasbo/tellulf-v9/internal/weather"
	"github.com/kvasbo/tellulf-v9/internal/web"
)

//go:embed static public
var embedded embed.FS

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	loadDotEnv()
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	src := web.Sources{
		Weather: weather.New(),
		Entur:   entur.New(),
	}
	var wg sync.WaitGroup
	wg.Go(func() { src.Weather.Run(ctx) })
	wg.Go(func() { src.Entur.Run(ctx) })

	if cal, err := newCalendar(ctx); err != nil {
		slog.Warn("calendar disabled", "reason", err)
	} else {
		src.Calendar = cal
		wg.Go(func() { cal.Run(ctx) })
	}

	if host := os.Getenv("MQTT_HOST"); host == "" {
		slog.Warn("sensors disabled", "reason", "MQTT_HOST is not set")
	} else {
		sh := smarthouse.New()
		if err := sh.Connect(smarthouse.Config{Host: host, User: os.Getenv("MQTT_USER"), Password: os.Getenv("MQTT_PASS")}); err != nil {
			return err
		}
		defer sh.Close()
		src.Smarthouse = sh
	}

	if cfg, err := tibberConfig(); err != nil {
		slog.Warn("power disabled", "reason", err)
	} else {
		t := tibber.New(cfg)
		src.Tibber = t
		wg.Go(func() { t.Run(ctx) })
	}

	// TELLULF_DEV=1 serves static files straight from disk, so CSS and
	// JavaScript edits show up on reload without rebuilding.
	var assets fs.FS = embedded
	if os.Getenv("TELLULF_DEV") != "" {
		assets = os.DirFS(".")
		slog.Info("dev mode: serving static files from disk")
	}
	server := web.New(src, assets)
	wg.Go(func() { server.Publish(ctx) })

	port := os.Getenv("EXPOSE_PORT")
	if port == "" {
		port = "3000"
	}
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Requests inherit ctx, so open SSE streams end on shutdown.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	slog.Info("Tellulf running", "port", port)
	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	wg.Wait()
	slog.Info("stopped")
	return nil
}

// loadDotEnv reads .env from the working directory. Variables already set
// in the environment win, so Docker/compose settings are never overridden.
// A missing file is fine.
func loadDotEnv() {
	err := godotenv.Load()
	switch {
	case err == nil:
		slog.Info("loaded .env")
	case !errors.Is(err, fs.ErrNotExist):
		slog.Warn("could not read .env", "err", err)
	}
}

func newCalendar(ctx context.Context) (*calendar.Calendar, error) {
	cfg := calendar.Config{
		Felles:       os.Getenv("CAL_ID_FELLES"),
		Audun:        os.Getenv("CAL_ID_AUDUN"),
		Barneuker:    os.Getenv("CAL_ID_BARNEUKER"),
		Bursdag:      os.Getenv("CAL_ID_BURSDAG"),
		Middag:       os.Getenv("CAL_ID_MIDDAG"),
		Kindergarden: os.Getenv("CAL_ID_KINDERGARDEN"),
	}
	if cfg == (calendar.Config{}) {
		return nil, errors.New("no CAL_ID_* variables are set")
	}
	encoded := os.Getenv("GOOGLE_KEY_B64")
	if encoded == "" {
		return nil, errors.New("GOOGLE_KEY_B64 is not set")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("GOOGLE_KEY_B64 is not valid base64")
	}
	return calendar.New(ctx, cfg, key)
}

func tibberConfig() (tibber.Config, error) {
	cfg := tibber.Config{
		Token:   os.Getenv("TIBBER_KEY"),
		HomeID:  os.Getenv("TIBBER_ID_HOME"),
		CabinID: os.Getenv("TIBBER_ID_CABIN"),
	}
	if cfg.Token == "" || cfg.HomeID == "" || cfg.CabinID == "" {
		return cfg, errors.New("TIBBER_KEY, TIBBER_ID_HOME and TIBBER_ID_CABIN must all be set")
	}
	return cfg, nil
}
