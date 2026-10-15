package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleForecast = `{"properties":{"timeseries":[
 {"time":"2026-10-10T06:00:00Z","data":{
   "instant":{"details":{"air_temperature":4.5,"wind_speed":6.2,"wind_from_direction":40}},
   "next_1_hours":{"summary":{"symbol_code":"heavyrain"},"details":{"precipitation_amount":2.5,"probability_of_precipitation":100}}}},
 {"time":"2026-10-10T07:00:00Z","data":{
   "instant":{"details":{}},
   "next_1_hours":{"summary":{"symbol_code":"cloudy"},"details":{"precipitation_amount":0,"probability_of_precipitation":0}}}},
 {"time":"2026-10-12T00:00:00Z","data":{"instant":{"details":{"air_temperature":1}}}}
]}}`

const sampleLongTerm = `{"properties":{"timeseries":[
 {"time":"2026-10-11T00:00:00Z","data":{"next_24_hours":{"details":{
   "air_temperature_max":5.3,"air_temperature_min":3.3,
   "probability_of_precipitation":64,"probability_of_heavy_precipitation":15}}}}
]}}`

func fakeMET(t *testing.T) *Weather {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "MET requires a User-Agent", http.StatusForbidden)
			return
		}
		if strings.Contains(r.URL.Path, "subseasonal") {
			w.Write([]byte(sampleLongTerm))
		} else {
			w.Write([]byte(sampleForecast))
		}
	}))
	t.Cleanup(srv.Close)
	w := New()
	w.baseURL = srv.URL
	return w
}

func TestHourlyForecasts(t *testing.T) {
	w := fakeMET(t)
	if err := w.updateForecast(context.Background(), Oslo); err != nil {
		t.Fatal(err)
	}
	got := w.HourlyForecasts(Oslo, time.Date(2026, 10, 10, 6, 30, 0, 0, time.UTC))
	if len(got) != 2 { // the entry without next_1_hours is skipped
		t.Fatalf("got %d hours, want 2", len(got))
	}
	first := got[0]
	if first.Hour != 8 || first.Symbol != "heavyrain" || first.PrecipitationAmount != 2.5 || *first.AirTemperature != 4.5 {
		t.Errorf("first hour = %+v", first) // 06:00Z is 08:00 in Oslo (CEST)
	}
	if got[1].AirTemperature != nil || got[1].WindSpeed != nil {
		t.Errorf("missing values should stay nil, got %+v", got[1])
	}
	if len(w.HourlyForecasts(Hytta, time.Time{})) != 0 {
		t.Error("Hytta has not been fetched yet")
	}

	// Later, the hours that have passed drop out.
	later := w.HourlyForecasts(Oslo, time.Date(2026, 10, 10, 7, 5, 0, 0, time.UTC))
	if len(later) != 1 || later[0].Hour != 9 {
		t.Errorf("at 07:05Z got %+v, want only the 07:00Z hour", later)
	}
}

func TestFollowsMETCacheHeaders(t *testing.T) {
	const lastModified = "Sat, 10 Oct 2026 06:00:00 GMT"
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("If-Modified-Since") == lastModified {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lastModified)
		w.Write([]byte(sampleForecast))
	}))
	defer srv.Close()
	w := New()
	w.baseURL = srv.URL
	ctx := context.Background()

	if err := w.updateForecast(ctx, Oslo); err != nil {
		t.Fatal(err)
	}
	// No Expires was sent, so ask again, with If-Modified-Since; the 304
	// keeps the forecast.
	if err := w.updateForecast(ctx, Oslo); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(w.forecast[Oslo]) != 3 {
		t.Errorf("requests = %d, series = %d; want 2 and 3", requests, len(w.forecast[Oslo]))
	}

	// Before Expires, don't ask at all.
	url := srv.URL + "/locationforecast/2.0/complete?lat=59.9508&lon=10.6848"
	w.cache[url] = cacheInfo{expires: time.Now().Add(time.Hour)}
	if err := w.updateForecast(ctx, Oslo); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("fetched before Expires: %d requests", requests)
	}
}

func TestDailyForecasts(t *testing.T) {
	w := fakeMET(t)
	w.updateLongTerm(context.Background(), Oslo)
	d, ok := w.DailyForecasts(Oslo)["2026-10-11"]
	if !ok {
		t.Fatal("no forecast for 2026-10-11")
	}
	want := Daily{MinTemp: 3.3, MaxTemp: 5.3, LightRainProbability: 60, HeavyRainProbability: 20}
	if d != want {
		t.Errorf("got %+v, want %+v", d, want)
	}
}
