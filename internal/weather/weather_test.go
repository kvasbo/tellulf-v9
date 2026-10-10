package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	got := w.HourlyForecasts(Oslo)
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
	if len(w.HourlyForecasts(Hytta)) != 0 {
		t.Error("Hytta has not been fetched yet")
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
