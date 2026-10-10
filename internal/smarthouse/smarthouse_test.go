package smarthouse

import (
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func TestHandle(t *testing.T) {
	s := New()
	s.handle("tellulf/weather/tempOut", "-3.5")
	s.handle("tellulf/weather/humidity", " 81.2\n")
	s.handle("tellulf/weather/pressure", "1013")
	s.handle("tellulf/weather/tempOutTime", "10-03-2026 14:05")
	s.handle("tellulf/weather/pressure", "garbage") // ignored
	s.handle("tellulf/other", "1")                  // ignored

	got := s.Readings()
	want := Readings{
		TempOut:      -3.5,
		HumOut:       81.2,
		Pressure:     1013,
		LastTempTime: time.Date(2026, 3, 10, 14, 5, 0, 0, tz.Oslo),
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBrokerURL(t *testing.T) {
	cases := map[string]string{
		"mqtt://broker":       "mqtt://broker:1883",
		"mqtt://broker:1884":  "mqtt://broker:1884",
		"broker":              "mqtt://broker:1883",
		"mqtts://broker":      "mqtts://broker:8883",
		"tcp://10.0.0.2:1883": "tcp://10.0.0.2:1883",
	}
	for in, want := range cases {
		if got, err := brokerURL(in); err != nil || got != want {
			t.Errorf("%q: got %q (%v), want %q", in, got, err, want)
		}
	}
}
