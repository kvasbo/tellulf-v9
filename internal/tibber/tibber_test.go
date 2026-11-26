package tibber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func ptr(f float64) *float64 { return &f }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// fakeTibber serves the GraphQL endpoint and a graphql-transport-ws endpoint
// that acks, pings, sends one measurement and then restarts ("going away").
func fakeTibber(t *testing.T, token string) *graphQL {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/gql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("User-Agent") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"viewer": map[string]any{
			"websocketSubscriptionUrl": wsURL,
			"home":                     map[string]any{"features": map[string]any{"realTimeConsumptionEnabled": true}},
		}}})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"graphql-transport-ws"}})
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if c.Subprotocol() != "graphql-transport-ws" {
			c.Close(4400, "wrong subprotocol")
			return
		}

		var init struct {
			Type    string            `json:"type"`
			Payload map[string]string `json:"payload"`
		}
		if wsjson.Read(ctx, c, &init); init.Type != "connection_init" || init.Payload["token"] != token {
			c.Close(4403, "Forbidden")
			return
		}
		wsjson.Write(ctx, c, map[string]string{"type": "connection_ack"})

		var sub wsMessage
		wsjson.Read(ctx, c, &sub)
		if sub.Type != "subscribe" || !strings.Contains(string(sub.Payload), "liveMeasurement") {
			t.Errorf("unexpected subscribe message %+v", sub)
		}

		wsjson.Write(ctx, c, map[string]string{"type": "ping"})
		var pong wsMessage
		if wsjson.Read(ctx, c, &pong); pong.Type != "pong" {
			t.Errorf("expected pong, got %+v", pong)
		}

		wsjson.Write(ctx, c, map[string]any{"id": sub.ID, "type": "next", "payload": map[string]any{
			"data": map[string]any{"liveMeasurement": map[string]any{
				"timestamp": "2026-10-10T12:00:00+02:00", "power": 1234.0,
				"accumulatedConsumption": 5.5, "accumulatedProduction": 0.5,
				"minPower": 100.0, "averagePower": 900.0, "maxPower": 4000.0,
				"accumulatedCost": nil, "powerProduction": nil,
			}},
		}})
		c.Close(websocket.StatusGoingAway, "Going away")
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &graphQL{http: srv.Client(), endpoint: srv.URL + "/gql", token: token}
}

func TestFeedProtocol(t *testing.T) {
	gql := fakeTibber(t, "secret")
	var got []Measurement
	f := &feed{gql: gql, homeID: "h1", onMessage: func(m Measurement) { got = append(got, m) }}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gotData, err := f.connectOnce(ctx)
	if !gotData || err != errGoingAway {
		t.Fatalf("connectOnce = %v, %v; want data and errGoingAway", gotData, err)
	}
	if len(got) != 1 || got[0].Power != 1234 || got[0].PowerProduction != nil {
		t.Errorf("got measurements %+v", got)
	}
}

func TestFeedRejectedToken(t *testing.T) {
	gql := fakeTibber(t, "secret")
	gql.token = "wrong"
	f := &feed{gql: gql, homeID: "h1", onMessage: func(Measurement) {}}
	if _, err := f.connectOnce(context.Background()); err != ErrUnauthorized {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}

func TestReconnectPlan(t *testing.T) {
	other := errors.New("network trouble")
	within := func(d time.Duration, attempt int) bool {
		ceiling := min(schedule.BaseBackoff<<min(attempt-1, 20), schedule.MaxBackoff)
		return d >= ceiling/2 && d <= ceiling
	}

	if _, _, stop := reconnectPlan(3, true, ErrUnauthorized); !stop {
		t.Error("a rejected token must stop the feed")
	}
	if next, d, _ := reconnectPlan(3, false, errNoDevice); next != 3 || d != noDeviceRecheck {
		t.Errorf("no device: got %d, %v", next, d)
	}
	if next, d, _ := reconnectPlan(4, true, errGoingAway); next != 0 || d < time.Second || d > time.Minute {
		t.Errorf("going away after data: got %d, %v", next, d)
	}
	if next, d, _ := reconnectPlan(4, true, other); next != 1 || !within(d, 1) {
		t.Errorf("a working connection dropping should start over: got %d, %v", next, d)
	}
	if next, d, _ := reconnectPlan(2, false, other); next != 3 || !within(d, 3) {
		t.Errorf("repeated failures should back off: got %d, %v", next, d)
	}

	// Being kicked as a duplicate keeps growing the wait, even when data
	// arrived first, so two clients don't fight over the feed.
	attempt := 0
	for i := 1; i <= 5; i++ {
		var d time.Duration
		attempt, d, _ = reconnectPlan(attempt, true, errDuplicate)
		if attempt != i || !within(d, i) {
			t.Errorf("duplicate #%d: got attempt %d, delay %v", i, attempt, d)
		}
	}
}

func TestClassifyDuplicateConnection(t *testing.T) {
	err := websocket.CloseError{Code: 4429, Reason: "duplicate connection"}
	if got := classify(fmt.Errorf("failed to get reader: %w", err)); !errors.Is(got, errDuplicate) {
		t.Errorf("got %v, want errDuplicate", got)
	}
}

func TestClientHours(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"home":{"consumption":{"nodes":[
			{"from":"2026-11-26T10:00:00+01:00","consumption":1.5},
			{"from":"2026-11-26T11:00:00+01:00","consumption":null},
			{"from":"2026-11-26T09:00:00+01:00","consumption":2}]}}}}}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("tok", "h1")
	c.gql.endpoint = srv.URL
	got, err := c.Hours(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	ten := time.Date(2026, 11, 26, 10, 0, 0, 0, tz.Oslo)
	if len(got) != 2 || !got[0].Start.Equal(ten) || got[0].KWh != 1.5 || got[1].KWh != 2 {
		t.Errorf("got %+v", got)
	}
}
