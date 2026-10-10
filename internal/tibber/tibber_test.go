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
	var got []liveMeasurement
	f := &feed{gql: gql, place: Home, homeID: "h1", onMessage: func(_ Place, m liveMeasurement) { got = append(got, m) }}

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
	f := &feed{gql: gql, place: Home, homeID: "h1", onMessage: func(Place, liveMeasurement) {}}
	if _, err := f.connectOnce(context.Background()); err != errUnauthorized {
		t.Errorf("got %v, want errUnauthorized", err)
	}
}

func TestBackoffGrowsAndStaysBounded(t *testing.T) {
	for attempt := 1; attempt < 40; attempt++ {
		d := backoff(attempt)
		ceiling := min(baseBackoff<<min(attempt-1, 20), maxBackoff)
		if d < ceiling/2 || d > ceiling {
			t.Errorf("attempt %d: %v outside [%v, %v]", attempt, d, ceiling/2, ceiling)
		}
	}
}

func TestAccumulatedCostCrossesCap(t *testing.T) {
	n := Norgespris{Prices: []PricePeriod{{Price: 0.5}}, HomeCap: 10, CabinCap: 5}
	hours := []hourlyUsage{
		{Consumption: 6, Price: 2}, // all subsidized: 3.0
		{Consumption: 6, Price: 2}, // 4 subsidized + 2 at spot: 2.0 + 4.0
		{Consumption: 1, Price: 3}, // over cap: 3.0
	}
	if got := n.AccumulatedCost(Home, hours); !near(got, 12) {
		t.Errorf("got %v, want 12", got)
	}
}

func TestOnMeasurementSplitsTodayAtTheCap(t *testing.T) {
	tb := New(Config{})
	tb.norgespris = Norgespris{Prices: []PricePeriod{{Price: 0.5}}, HomeCap: 100, CabinCap: 10}
	tb.data[Home].Cap = 100
	tb.data[Home].CurrentPrice = 2
	tb.monthBeforeToday[Home] = 95
	tb.monthCostBeforeToday[Home] = 40

	tb.onMeasurement(Home, liveMeasurement{Power: 800, AccumulatedConsumption: 10})
	d := tb.PowerData(Home)
	// 5 kWh left under the cap at 0.5, 5 kWh at spot 2.
	if !near(d.AccumulatedCost, 12.5) || !near(d.MonthlyConsumption, 105) || !near(d.MonthlyCost, 52.5) {
		t.Errorf("got cost %v, monthly %v, monthly cost %v", d.AccumulatedCost, d.MonthlyConsumption, d.MonthlyCost)
	}
	if d.EffectivePrice != 2 {
		t.Errorf("over the cap the effective price is spot, got %v", d.EffectivePrice)
	}
}

func TestCabinShowsProductionAsNegativePower(t *testing.T) {
	tb := New(Config{})
	tb.onMeasurement(Cabin, liveMeasurement{Power: 0, PowerProduction: ptr(1500)})
	if got := tb.PowerData(Cabin).CurrentPower; got != -1500 {
		t.Errorf("got %v, want -1500", got)
	}
	// A frame without production keeps using the last known value.
	tb.onMeasurement(Cabin, liveMeasurement{Power: 0})
	if got := tb.PowerData(Cabin).CurrentPower; got != -1500 {
		t.Errorf("got %v, want -1500", got)
	}
}

func TestMonthHoursBeforeToday(t *testing.T) {
	now := time.Date(2026, 10, 3, 14, 0, 0, 0, tz.Oslo)
	at := func(m time.Month, d, h int) time.Time { return time.Date(2026, m, d, h, 0, 0, 0, tz.Oslo) }
	nodes := []consumptionNode{
		{From: at(10, 2, 5), Consumption: ptr(2), Cost: ptr(3)},
		{From: at(9, 30, 23), Consumption: ptr(9), Cost: ptr(9)}, // last month
		{From: at(10, 1, 0), Consumption: ptr(1), Cost: ptr(1)},
		{From: at(10, 3, 1), Consumption: ptr(9), Cost: ptr(9)}, // today
		{From: at(10, 2, 6), Consumption: nil, Cost: nil},       // not yet known
	}
	got := monthHoursBeforeToday(nodes, now)
	if len(got) != 3 || !got[0].Start.Equal(at(10, 1, 0)) || got[1].Price != 1.5 || got[2].Consumption != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestNorgesprisPriceChangesOnNewYear2027(t *testing.T) {
	n := defaultNorgespris
	oslo := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, tz.Oslo) }
	cases := []struct {
		at   time.Time
		want float64
	}{
		{oslo(2025, 10, 1, 0, 0), 0.50},
		{oslo(2026, 12, 31, 23, 59), 0.50},
		{oslo(2027, 1, 1, 0, 0), 0.5625}, // 45 øre + 25 % VAT
		{oslo(2027, 6, 1, 12, 0), 0.5625},
	}
	for _, c := range cases {
		if got := n.PriceAt(c.at); !near(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.at, got, c.want)
		}
		if got := n.EffectivePrice(Home, c.at, 100, 3); !near(got, c.want) {
			t.Errorf("%s under the cap: got %v, want %v", c.at, got, c.want)
		}
	}
	if got := n.EffectivePrice(Home, oslo(2027, 1, 5, 12, 0), 5000, 3); got != 3 {
		t.Errorf("at the cap the spot price applies, got %v", got)
	}

	// Each hour is priced at the rate in effect when it was used.
	hours := []hourlyUsage{
		{Start: oslo(2026, 12, 31, 23, 0), Consumption: 2, Price: 1},
		{Start: oslo(2027, 1, 1, 0, 0), Consumption: 2, Price: 1},
	}
	if got := n.AccumulatedCost(Home, hours); !near(got, 2*0.5+2*0.5625) {
		t.Errorf("cost across the change: got %v", got)
	}
}

func TestReconnectPlan(t *testing.T) {
	other := errors.New("network trouble")
	within := func(d time.Duration, attempt int) bool {
		ceiling := min(baseBackoff<<min(attempt-1, 20), maxBackoff)
		return d >= ceiling/2 && d <= ceiling
	}

	if _, _, stop := reconnectPlan(3, true, errUnauthorized); !stop {
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
