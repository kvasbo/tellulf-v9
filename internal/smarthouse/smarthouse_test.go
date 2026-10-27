package smarthouse

import (
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/kvasbo/tellulf-v9/internal/mode"
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
	if got.TempOut == nil || *got.TempOut != -3.5 ||
		got.HumOut == nil || *got.HumOut != 81.2 ||
		got.Pressure == nil || *got.Pressure != 1013 ||
		!got.LastTempTime.Equal(time.Date(2026, 3, 10, 14, 5, 0, 0, tz.Oslo)) {
		t.Errorf("got %+v", got)
	}

	if r := New().Readings(); r.TempOut != nil || r.HumOut != nil || r.Pressure != nil {
		t.Errorf("readings before any message should be nil, got %+v", r)
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

func TestHandleMode(t *testing.T) {
	s := New()
	if m, since := s.Mode(); m != mode.Oversikt || !since.IsZero() {
		t.Errorf("mode before any message = %q at %v, want the overview", m, since)
	}
	steps := []struct {
		payload string
		want    mode.Mode
	}{
		{"strom", mode.Strom},
		{"natt", mode.Strom}, // unknown: ignored
		{" /kalender\n", mode.Kalender},
		{"", mode.Oversikt},
	}
	for _, st := range steps {
		s.handle("tellulf/mode", st.payload)
		if got, _ := s.Mode(); got != st.want {
			t.Errorf("after %q: mode = %q, want %q", st.payload, got, st.want)
		}
	}
}

// Every message, also for the page already shown, restarts the clock.
func TestModeRecordsWhen(t *testing.T) {
	s := New()
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, tz.Oslo)
	s.now = func() time.Time { return at }
	s.handle("tellulf/mode", "strom")
	at = at.Add(90 * time.Second)
	s.handle("tellulf/mode", "strom")
	if m, since := s.Mode(); m != mode.Strom || !since.Equal(at) {
		t.Errorf("got %q at %v, want strom at %v", m, since, at)
	}
}

// fakeMessage is the part of an MQTT message onMessage reads.
type fakeMessage struct {
	mqtt.Message
	topic, payload string
	retained       bool
}

func (m fakeMessage) Topic() string   { return m.topic }
func (m fakeMessage) Payload() []byte { return []byte(m.payload) }
func (m fakeMessage) Retained() bool  { return m.retained }

// The broker replays a retained mode on every reconnect; it must not bring
// an old page back.
func TestRetainedModeIsIgnored(t *testing.T) {
	s := New()
	s.onMessage(nil, fakeMessage{topic: "tellulf/mode", payload: "kalender", retained: true})
	if got, _ := s.Mode(); got != mode.Oversikt {
		t.Errorf("after a retained message: %q, want the overview", got)
	}
	s.onMessage(nil, fakeMessage{topic: "tellulf/mode", payload: "kalender"})
	if got, _ := s.Mode(); got != mode.Kalender {
		t.Errorf("after a live message: %q, want kalender", got)
	}
	// Retained sensor readings still count.
	s.onMessage(nil, fakeMessage{topic: "tellulf/weather/tempOut", payload: "4", retained: true})
	if r := s.Readings(); r.TempOut == nil || *r.TempOut != 4 {
		t.Errorf("retained temperature ignored: %+v", r)
	}
}
