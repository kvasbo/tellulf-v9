package tibber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
)

// The live feed speaks the graphql-transport-ws protocol:
// https://github.com/enisdenjo/graphql-ws/blob/master/PROTOCOL.md
//
//	client → connection_init {token}     server → connection_ack
//	client → subscribe {query}           server → next {data} ... (repeated)
//	either → ping / pong                 server → error / complete
//
// Tibber allows two open websockets per token, i.e. exactly one per home, so
// each feed runs one connection at a time and fully closes it before
// reconnecting.

const liveQuery = `subscription($homeId: ID!) {
  liveMeasurement(homeId: $homeId) {
    timestamp power
    accumulatedConsumption accumulatedProduction
    minPower averagePower maxPower
    powerProduction maxPowerProduction
  }
}`

// liveMeasurement fields marked nullable in Tibber's schema are pointers.
type liveMeasurement struct {
	Timestamp              string   `json:"timestamp"`
	Power                  float64  `json:"power"`
	AccumulatedConsumption float64  `json:"accumulatedConsumption"`
	AccumulatedProduction  float64  `json:"accumulatedProduction"`
	MinPower               float64  `json:"minPower"`
	AveragePower           float64  `json:"averagePower"`
	MaxPower               float64  `json:"maxPower"`
	PowerProduction        *float64 `json:"powerProduction"`
	MaxPowerProduction     *float64 `json:"maxPowerProduction"`
}

type wsMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

const (
	ackTimeout = 30 * time.Second
	// Reconnect if a connection delivers nothing for this long.
	staleLimit = 5 * time.Minute
	// Reconnect delays grow exponentially (with jitter) from baseBackoff up to
	// maxBackoff while connections keep failing, and reset once data arrives.
	baseBackoff = 10 * time.Second
	maxBackoff  = 30 * time.Minute
	// How long to wait before asking again when the home has no Pulse/Watty.
	noDeviceRecheck = 30 * time.Minute
)

// errGoingAway is a Tibber server restart (close code 1001). Tibber asks for
// a random 1-60 second delay before reconnecting.
var errGoingAway = errors.New("tibber server going away")

// errDuplicate is close code 4429 ("duplicate connection"): Tibber already
// has a subscription for this home with the same token, e.g. a previous
// instance that hasn't been cleaned up yet, or another integration.
var errDuplicate = errors.New("tibber: duplicate connection")

// backoff returns the delay before reconnect attempt n (1-based): half fixed,
// half random, so clients don't reconnect in lockstep.
func backoff(attempt int) time.Duration {
	d := baseBackoff << min(attempt-1, 20)
	d = min(d, maxBackoff)
	return d/2 + rand.N(d/2)
}

type feed struct {
	gql       *graphQL
	place     Place
	homeID    string
	onMessage func(Place, liveMeasurement)
}

// run keeps the feed connected until ctx is cancelled or Tibber rejects the
// token.
func (f *feed) run(ctx context.Context) {
	log := slog.With("place", f.place)
	attempt := 0
	for {
		gotData, err := f.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		var delay time.Duration
		var stop bool
		attempt, delay, stop = reconnectPlan(attempt, gotData, err)
		wait := delay.Round(time.Second)
		switch {
		case stop:
			log.Error("Tibber rejected the token; live feed stopped until restart", "err", err)
			return
		case errors.Is(err, errNoDevice):
			log.Warn("home has no real-time device; checking again later", "in", wait)
		case errors.Is(err, errGoingAway):
			log.Info("Tibber feed restarting", "reconnect_in", wait)
		case errors.Is(err, errDuplicate):
			log.Warn("Tibber closed the feed as a duplicate: another connection for this home uses the same token",
				"reconnect_in", wait)
		default:
			log.Warn("Tibber feed disconnected", "err", err, "reconnect_in", wait)
		}
		if !schedule.Sleep(ctx, delay) {
			return
		}
	}
}

// reconnectPlan decides what to do after a connection ends. attempt counts
// consecutive failed connections; it returns the new count, how long to
// wait before reconnecting, and whether to give up.
func reconnectPlan(attempt int, gotData bool, err error) (next int, delay time.Duration, stop bool) {
	switch {
	case errors.Is(err, errUnauthorized):
		return attempt, 0, true
	case errors.Is(err, errNoDevice):
		return attempt, noDeviceRecheck, false
	case errors.Is(err, errDuplicate):
		// Another client holds this home's feed. Even if we got data before
		// being kicked, keep backing off so two clients don't take turns
		// kicking each other every few seconds.
		return attempt + 1, backoff(attempt + 1), false
	}
	if gotData {
		attempt = 0 // the connection worked; start the backoff over
	}
	if errors.Is(err, errGoingAway) {
		return attempt, time.Second + rand.N(59*time.Second), false
	}
	return attempt + 1, backoff(attempt + 1), false
}

var errNoDevice = errors.New("no real-time device")

// connectOnce runs one websocket connection from dial to close. gotData says
// whether any measurement arrived before it ended.
func (f *feed) connectOnce(ctx context.Context) (gotData bool, err error) {
	url, realTime, err := f.gql.feedInfo(ctx, f.homeID)
	if err != nil {
		return false, err
	}
	if !realTime {
		return false, errNoDevice
	}

	dialCtx, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()
	conn, res, err := websocket.Dial(dialCtx, url, &websocket.DialOptions{
		Subprotocols: []string{"graphql-transport-ws"},
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + f.gql.token},
			"User-Agent":    {userAgent},
		},
	})
	if err != nil {
		if res != nil && (res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden) {
			return false, errUnauthorized
		}
		return false, fmt.Errorf("dial: %w", err)
	}
	// Always destroy the connection on the way out, whatever happened.
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)

	if err := wsjson.Write(dialCtx, conn, map[string]any{
		"type":    "connection_init",
		"payload": map[string]string{"token": f.gql.token},
	}); err != nil {
		return false, classify(err)
	}
	if err := f.awaitAck(dialCtx, conn); err != nil {
		return false, err
	}
	slog.Info("connected to Tibber", "place", f.place)

	sub, _ := json.Marshal(map[string]any{
		"query":     liveQuery,
		"variables": map[string]string{"homeId": f.homeID},
	})
	if err := wsjson.Write(ctx, conn, wsMessage{ID: "1", Type: "subscribe", Payload: sub}); err != nil {
		return false, classify(err)
	}

	for {
		msg, err := f.read(ctx, conn, staleLimit)
		if err != nil {
			if ctx.Err() != nil {
				conn.Close(websocket.StatusNormalClosure, "shutting down")
			}
			return gotData, err
		}
		switch msg.Type {
		case "next":
			var p struct {
				Data *struct {
					LiveMeasurement *liveMeasurement `json:"liveMeasurement"`
				} `json:"data"`
				Errors []gqlError `json:"errors"`
			}
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				return gotData, fmt.Errorf("bad next payload: %w", err)
			}
			if len(p.Errors) > 0 {
				slog.Warn("Tibber feed error", "place", f.place, "errors", p.Errors)
			}
			if p.Data != nil && p.Data.LiveMeasurement != nil {
				gotData = true
				f.onMessage(f.place, *p.Data.LiveMeasurement)
			}
		case "error":
			return gotData, fmt.Errorf("subscription error: %s", msg.Payload)
		case "complete":
			return gotData, errors.New("server completed the subscription")
		}
	}
}

func (f *feed) awaitAck(ctx context.Context, conn *websocket.Conn) error {
	for {
		msg, err := f.read(ctx, conn, ackTimeout)
		if err != nil {
			return err
		}
		if msg.Type == "connection_ack" {
			return nil
		}
	}
}

// read returns the next message that isn't a ping, answering pings on the way.
// It fails if nothing arrives within timeout.
func (f *feed) read(ctx context.Context, conn *websocket.Conn, timeout time.Duration) (wsMessage, error) {
	for {
		readCtx, cancel := context.WithTimeout(ctx, timeout)
		var msg wsMessage
		err := wsjson.Read(readCtx, conn, &msg)
		cancel()
		if err != nil {
			if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
				return msg, fmt.Errorf("no data for %v", timeout)
			}
			return msg, classify(err)
		}
		if msg.Type == "ping" {
			if err := wsjson.Write(ctx, conn, wsMessage{Type: "pong"}); err != nil {
				return msg, classify(err)
			}
			continue
		}
		return msg, nil
	}
}

// classify maps websocket close codes to the errors run() acts on.
func classify(err error) error {
	switch websocket.CloseStatus(err) {
	case websocket.StatusGoingAway:
		return errGoingAway
	case 4401, 4403: // graphql-transport-ws: Unauthorized / Forbidden
		return errUnauthorized
	case 4429:
		return errDuplicate
	}
	return err
}
