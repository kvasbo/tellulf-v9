// Package smarthouse listens on MQTT for outdoor sensor readings and for the
// mode that picks which page the screen shows.
package smarthouse

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/kvasbo/tellulf-v9/internal/mode"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Readings are the latest sensor values; nil until the sensor reports.
// Each update stores a fresh pointer, so copies of Readings never change.
type Readings struct {
	TempOut      *float64
	LastTempTime time.Time // zero until the sensor has reported a time
	HumOut       *float64
	Pressure     *float64
}

type Config struct {
	Host     string // e.g. mqtt://broker:1883
	User     string
	Password string
}

type Smarthouse struct {
	client mqtt.Client

	mu        sync.RWMutex
	data      Readings
	mode      mode.Mode // the overview until a message on modeTopic
	modeSince time.Time // when it was picked
	now       func() time.Time
}

// modeTopic carries the page every screen should show: a mode's name or
// its URL path (see mode.Parse). It holds for mode.Timeout. A retained
// message is ignored: the broker replays it on every reconnect, which would
// bring an old page back.
const modeTopic = "tellulf/mode"

func New() *Smarthouse {
	return &Smarthouse{mode: mode.Oversikt, now: time.Now}
}

// Connect starts the MQTT client. Paho reconnects on its own, both for the
// first connection and after drops.
func (s *Smarthouse) Connect(cfg Config) error {
	broker, err := brokerURL(cfg.Host)
	if err != nil {
		return err
	}
	clientID := fmt.Sprintf("tellulf-%06x", rand.IntN(1<<24))
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetUsername(cfg.User).
		SetPassword(cfg.Password).
		SetKeepAlive(15 * time.Second).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(c mqtt.Client) {
			slog.Info("MQTT connected", "broker", broker, "client_id", clientID)
			// Subscribe again on every (re)connect; the session is not persistent.
			c.Subscribe("tellulf/weather/#", 0, s.onMessage)
			c.Subscribe(modeTopic, 0, s.onMessage)
			c.Publish("tellulf/poll", 0, false, "Tellulf is online and polling")
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			slog.Warn("MQTT connection lost", "err", err)
		}).
		SetReconnectingHandler(func(mqtt.Client, *mqtt.ClientOptions) {
			slog.Info("MQTT reconnecting", "broker", broker)
		})

	slog.Info("connecting to MQTT", "broker", broker)
	s.client = mqtt.NewClient(opts)
	s.client.Connect() // with ConnectRetry this never fails; it keeps trying
	return nil
}

// Close disconnects from the broker, waiting briefly for in-flight work.
func (s *Smarthouse) Close() {
	if s.client != nil {
		s.client.Disconnect(250)
	}
}

// brokerURL accepts "host", "mqtt://host" or "mqtts://host:port", adding the
// default port when it's missing since paho needs one.
func brokerURL(host string) (string, error) {
	if !strings.Contains(host, "://") {
		host = "mqtt://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("invalid MQTT_HOST %q: %w", host, err)
	}
	if u.Port() == "" {
		port := "1883"
		switch u.Scheme {
		case "mqtts", "ssl", "tls", "tcps", "mqtt+ssl":
			port = "8883"
		}
		u.Host = net.JoinHostPort(u.Hostname(), port)
	}
	return u.String(), nil
}

// Readings returns the latest sensor values.
func (s *Smarthouse) Readings() Readings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Mode returns the page every screen should show: the one MQTT last asked
// for, back to the overview once it has been up for mode.Timeout.
func (s *Smarthouse) Mode() mode.Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return mode.Current(s.mode, s.modeSince, s.now())
}

func (s *Smarthouse) onMessage(_ mqtt.Client, msg mqtt.Message) {
	if msg.Topic() == modeTopic && msg.Retained() {
		slog.Info("MQTT: ignoring a retained mode; publish modes without retain", "payload", string(msg.Payload()))
		return
	}
	s.handle(msg.Topic(), string(msg.Payload()))
}

func (s *Smarthouse) handle(topic, payload string) {
	payload = strings.TrimSpace(payload)
	s.mu.Lock()
	defer s.mu.Unlock()

	if topic == modeTopic {
		m, ok := mode.Parse(payload)
		if !ok {
			slog.Warn("MQTT: unknown mode", "payload", payload)
			return
		}
		slog.Info("mode set", "mode", m)
		s.mode, s.modeSince = m, s.now()
		return
	}

	if topic == "tellulf/weather/tempOutTime" {
		t, err := time.ParseInLocation("02-01-2006 15:04", payload, tz.Oslo)
		if err != nil {
			slog.Warn("MQTT: bad temperature time", "payload", payload, "err", err)
			return
		}
		s.data.LastTempTime = t
		slog.Debug("MQTT temperature time set", "time", t)
		return
	}

	var target **float64
	switch topic {
	case "tellulf/weather/tempOut":
		target = &s.data.TempOut
	case "tellulf/weather/humidity":
		target = &s.data.HumOut
	case "tellulf/weather/pressure":
		target = &s.data.Pressure
	default:
		return
	}
	v, err := strconv.ParseFloat(payload, 64)
	if err != nil {
		slog.Warn("MQTT: bad number", "topic", topic, "payload", payload)
		return
	}
	*target = &v
	slog.Debug("MQTT value set", "topic", topic, "value", v)
}
