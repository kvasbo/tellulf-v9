package entur

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeCall struct {
	Realtime              bool   `json:"realtime"`
	AimedDepartureTime    string `json:"aimedDepartureTime"`
	ExpectedDepartureTime string `json:"expectedDepartureTime"`
	DestinationDisplay    struct {
		FrontText string `json:"frontText"`
	} `json:"destinationDisplay"`
	ServiceJourney struct {
		DirectionType string `json:"directionType"`
	} `json:"serviceJourney"`
}

func call(at, destination, direction string) fakeCall {
	c := fakeCall{Realtime: true, AimedDepartureTime: at, ExpectedDepartureTime: at}
	c.DestinationDisplay.FrontText = destination
	c.ServiceJourney.DirectionType = direction
	return c
}

func graphql(calls ...fakeCall) map[string]any {
	return map[string]any{"data": map[string]any{"stopPlace": map[string]any{"estimatedCalls": calls}}}
}

// fakeEntur serves whatever body points to; tests swap it between updates.
func fakeEntur(t *testing.T, body *any) *Entur {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(*body)
	}))
	t.Cleanup(srv.Close)
	e := New()
	e.endpoint = srv.URL
	return e
}

func TestKeepsOnlyInboundSortedByTime(t *testing.T) {
	var body any = graphql(
		call("2026-08-24T11:47:00+02:00", "Bergkrystallen", "inbound"),
		call("2026-08-24T11:37:00+02:00", "Frognerseteren", "outbound"),
		call("2026-08-24T11:32:00+02:00", "Bergkrystallen", "inbound"),
	)
	e := fakeEntur(t, &body)
	if err := e.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := e.Trains()
	if len(got) != 2 {
		t.Fatalf("got %d trains, want 2", len(got))
	}
	want := []string{"2026-08-24T11:32:00+02:00", "2026-08-24T11:47:00+02:00"}
	for i, tr := range got {
		w, _ := time.Parse(time.RFC3339, want[i])
		if !tr.Time.Equal(w) || tr.Destination != "Bergkrystallen" || !tr.Realtime || tr.Delayed {
			t.Errorf("train %d = %+v", i, tr)
		}
	}
}

func TestFlagsMissingRealtimeAndDelays(t *testing.T) {
	a := call("2026-08-24T11:32:00+02:00", "A", "inbound")
	a.Realtime = false
	b := call("2026-08-24T11:42:00+02:00", "B", "inbound")
	b.AimedDepartureTime = "2026-08-24T11:40:00+02:00" // exactly two minutes is not "more than"
	c := call("2026-08-24T11:52:01+02:00", "C", "inbound")
	c.AimedDepartureTime = "2026-08-24T11:50:00+02:00"

	var body any = graphql(a, b, c)
	e := fakeEntur(t, &body)
	if err := e.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		dest              string
		realtime, delayed bool
	}{{"A", false, false}, {"B", true, false}, {"C", true, true}}
	for i, tr := range e.Trains() {
		if tr.Destination != want[i].dest || tr.Realtime != want[i].realtime || tr.Delayed != want[i].delayed {
			t.Errorf("train %d = %+v, want %+v", i, tr, want[i])
		}
	}
}

func TestKeepsPreviousDeparturesOnInvalidResponse(t *testing.T) {
	var body any = graphql(
		call("2026-08-24T11:47:00+02:00", "Bergkrystallen", "inbound"),
		call("2026-08-24T11:32:00+02:00", "Bergkrystallen", "inbound"),
	)
	e := fakeEntur(t, &body)
	if err := e.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	body = map[string]any{"errors": []any{map[string]string{"message": "boom"}}}
	if err := e.Update(context.Background()); err == nil {
		t.Error("expected an error for a response without data")
	}
	if n := len(e.Trains()); n != 2 {
		t.Errorf("got %d trains, want the previous 2", n)
	}
}
