// Package weather keeps forecasts from MET Norway (yr.no) for Oslo and the
// cabin up to date.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

type Location string

const (
	Oslo  Location = "oslo"
	Hytta Location = "hytta"
)

type coords struct{ lat, lon float64 }

var places = map[Location]coords{
	Oslo:  {59.9508, 10.6848},
	Hytta: {59.1347, 10.3246},
}

// Hourly is one hour of the short-term forecast.
type Hourly struct {
	Hour                       int // hour of day in Oslo, 0-23
	Symbol                     string
	PrecipitationAmount        float64
	ProbabilityOfPrecipitation float64
	AirTemperature             *float64
	WindSpeed                  *float64
	WindSpeedOfGust            *float64
	WindFromDirection          *float64
}

// Daily is one day of the long-term (subseasonal) forecast.
type Daily struct {
	MinTemp              float64
	MaxTemp              float64
	LightRainProbability float64 // percent, rounded to nearest 10
	HeavyRainProbability float64
}

const (
	userAgent       = "tellulf v9: audun@kvasbo.no"
	refreshInterval = 30 * time.Minute
	// yr.no throttles aggressive clients, so failed fetches back off
	// exponentially from baseRetry up to maxRetry.
	baseRetry = 10 * time.Second
	maxRetry  = 5 * time.Minute
)

type Weather struct {
	client  *http.Client
	baseURL string

	mu       sync.RWMutex
	forecast map[Location][]timeSeries
	longTerm map[Location][]longTermDay
}

func New() *Weather {
	return &Weather{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  "https://api.met.no/weatherapi",
		forecast: map[Location][]timeSeries{},
		longTerm: map[Location][]longTermDay{},
	}
}

// Run keeps every location's forecasts fresh until ctx is cancelled.
func (w *Weather) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for loc := range places {
		wg.Go(func() { w.runForecast(ctx, loc) })
		wg.Go(func() {
			schedule.Every(ctx, refreshInterval, func(ctx context.Context) { w.updateLongTerm(ctx, loc) })
		})
	}
	wg.Wait()
}

// runForecast fetches the hourly forecast every 30 minutes, retrying with
// exponential backoff when a fetch fails.
func (w *Weather) runForecast(ctx context.Context, loc Location) {
	retry := baseRetry
	for {
		next := refreshInterval
		if err := w.updateForecast(ctx, loc); err != nil {
			slog.Error("could not update forecast", "location", loc, "retry_in", retry, "err", err)
			next = retry
			retry = min(retry*2, maxRetry)
		} else {
			retry = baseRetry
		}
		if !schedule.Sleep(ctx, next) {
			return
		}
	}
}

func (w *Weather) updateForecast(ctx context.Context, loc Location) error {
	p := places[loc]
	var resp forecastResponse
	url := fmt.Sprintf("%s/locationforecast/2.0/complete?lat=%v&lon=%v", w.baseURL, p.lat, p.lon)
	if err := w.getJSON(ctx, url, &resp); err != nil {
		return err
	}
	series := resp.Properties.Timeseries
	if len(series) == 0 {
		return errors.New("forecast has no time series")
	}
	w.mu.Lock()
	w.forecast[loc] = series
	w.mu.Unlock()
	slog.Info("forecast updated", "location", loc, "series", len(series))
	return nil
}

func (w *Weather) updateLongTerm(ctx context.Context, loc Location) {
	p := places[loc]
	var resp longTermResponse
	url := fmt.Sprintf("%s/subseasonal/1.0/complete?lat=%v&lon=%v", w.baseURL, p.lat, p.lon)
	if err := w.getJSON(ctx, url, &resp); err != nil {
		slog.Error("could not update long-term forecast", "location", loc, "err", err)
		return
	}
	w.mu.Lock()
	w.longTerm[loc] = resp.Properties.Timeseries
	w.mu.Unlock()
	slog.Info("long-term forecast updated", "location", loc, "days", len(resp.Properties.Timeseries))
}

func (w *Weather) getJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(into)
}

// HourlyForecasts returns the hourly forecast for loc, starting with the
// current hour.
func (w *Weather) HourlyForecasts(loc Location) []Hourly {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []Hourly
	for _, s := range w.forecast[loc] {
		next := s.Data.Next1Hours
		if next == nil {
			continue
		}
		in := s.Data.Instant.Details
		out = append(out, Hourly{
			Hour:                       s.Time.In(tz.Oslo).Hour(),
			Symbol:                     next.Summary.SymbolCode,
			PrecipitationAmount:        next.Details.PrecipitationAmount,
			ProbabilityOfPrecipitation: next.Details.ProbabilityOfPrecipitation,
			AirTemperature:             in.AirTemperature,
			WindSpeed:                  in.WindSpeed,
			WindSpeedOfGust:            in.WindSpeedOfGust,
			WindFromDirection:          in.WindFromDirection,
		})
	}
	return out
}

// DailyForecasts returns the long-term forecast for loc keyed by date
// ("2006-01-02"). MET stamps each day at midnight UTC, so the UTC date is
// the day the forecast describes.
func (w *Weather) DailyForecasts(loc Location) map[string]Daily {
	w.mu.RLock()
	defer w.mu.RUnlock()
	roundTen := func(p float64) float64 { return tz.Round(p/10) * 10 }
	out := make(map[string]Daily, len(w.longTerm[loc]))
	for _, d := range w.longTerm[loc] {
		det := d.Data.Next24Hours.Details
		out[d.Time.UTC().Format(time.DateOnly)] = Daily{
			MinTemp:              det.AirTemperatureMin,
			MaxTemp:              det.AirTemperatureMax,
			LightRainProbability: roundTen(det.ProbabilityOfPrecipitation),
			HeavyRainProbability: roundTen(det.ProbabilityOfHeavyPrecipitation),
		}
	}
	return out
}
