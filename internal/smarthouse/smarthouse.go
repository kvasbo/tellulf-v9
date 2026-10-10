// Package smarthouse listens for outdoor sensor readings on MQTT.
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

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Readings are the latest sensor values. Until a sensor reports, its value
// is a -9999 sentinel, so a missing sensor is obvious on the display.
type Readings struct {
	TempOut      float64
	LastTempTime time.Time // zero until the sensor has reported a time
	HumOut       float64
	Pressure     float64
}

type Config struct {
	Host     string // e.g. mqtt://broker:1883
	User     string
	Password string
}

type Smarthouse struct {
	client mqtt.Client

	mu   sync.RWMutex
	data Readings
}

func New() *Smarthouse {
	return &Smarthouse{data: Readings{TempOut: -9999, HumOut: -9999}}
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

func (s *Smarthouse) onMessage(_ mqtt.Client, msg mqtt.Message) {
	s.handle(msg.Topic(), string(msg.Payload()))
}

func (s *Smarthouse) handle(topic, payload string) {
	payload = strings.TrimSpace(payload)
	s.mu.Lock()
	defer s.mu.Unlock()

	if topic == "tellulf/weather/tempOutTime" {
		t, err := time.ParseInLocation("02-01-2006 15:04", payload, tz.Oslo)
		if err != nil {
			slog.Warn("MQTT: bad temperature time", "payload", payload, "err", err)
			return
		}
		s.data.LastTempTime = t
		slog.Info("MQTT temperature time set", "time", t)
		return
	}

	var target *float64
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
	*target = v
	slog.Info("MQTT value set", "topic", topic, "value", v)
}
