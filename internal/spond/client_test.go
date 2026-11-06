package spond

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

var clientNow = time.Date(2026, 11, 6, 12, 0, 0, 0, tz.Oslo)

// The shapes below are trimmed from real answers: the fields the client
// reads, plus a few it doesn't.
const groupsJSON = `[
	{"id": "G1", "name": "G2018 - Fotball", "activity": "football", "members": [
		{"id": "M-JONAS", "firstName": "Jonas", "lastName": "K", "guardians": [{"id": "P1"}]},
		{"id": "M-OTHER", "firstName": "Noen", "lastName": "Andre", "guardians": [{"id": "P2"}]}
	]},
	{"id": "G2", "name": "TURN", "activity": "gymnastics", "members": [
		{"id": "M-AGNES", "firstName": "Agnes", "lastName": "K", "guardians": [{"id": "P1"}]}
	]}
]`

const eventsJSON = `[
	{"id": "E1", "heading": "TURN 5.-7. klasse", "type": "RECURRING", "behalfOfIds": ["M-AGNES"],
	 "startTimestamp": "2026-11-06T16:00:00Z", "endTimestamp": "2026-11-06T17:30:00Z", "expired": false,
	 "responses": {"acceptedIds": ["M-OTHER"], "declinedIds": [], "unansweredIds": ["M-AGNES"], "waitinglistIds": [], "unconfirmedIds": []}},
	{"id": "E2", "heading": "Heming – Grei", "type": "EVENT", "behalfOfIds": ["M-JONAS"], "matchEvent": true,
	 "description": "Ta med drikke", "location": {"id": "L1", "feature": "Slemdal kunstgress", "address": "Slemdalsveien 1"},
	 "meetupTimestamp": "2026-11-07T08:30:00Z", "rsvpDate": "2026-11-06T20:00:00Z",
	 "startTimestamp": "2026-11-07T09:00:00Z", "endTimestamp": "2026-11-07T10:00:00Z",
	 "responses": {"acceptedIds": ["M-JONAS"], "declinedIds": [], "unansweredIds": [], "waitinglistIds": [], "unconfirmedIds": []}},
	{"id": "E3", "heading": "Trening", "behalfOfIds": ["M-JONAS"],
	 "startTimestamp": "2026-11-08T09:00:00Z", "endTimestamp": "2026-11-08T10:00:00Z",
	 "responses": {"declinedIds": ["M-JONAS"]}},
	{"id": "E4", "heading": "Cup", "behalfOfIds": ["M-JONAS"],
	 "startTimestamp": "2026-11-09T09:00:00Z", "endTimestamp": "2026-11-09T10:00:00Z",
	 "responses": {"waitinglistIds": ["M-JONAS"]}},
	{"id": "E5", "heading": "Avlyst trening", "behalfOfIds": ["M-JONAS"], "cancelled": true, "cancelledReason": "Snø",
	 "startTimestamp": "2026-11-10T09:00:00Z", "endTimestamp": "2026-11-10T10:00:00Z",
	 "responses": {"unansweredIds": ["M-JONAS"]}},
	{"id": "E6", "heading": "Dugnad for voksne", "behalfOfIds": [],
	 "startTimestamp": "2026-11-11T09:00:00Z", "endTimestamp": "2026-11-11T10:00:00Z", "responses": {}}
]`

// fakeSpond counts the calls and answers like the real API; the tests turn
// parts of it off to see what the client does.
type fakeSpond struct {
	eventsBody             string // instead of eventsJSON, if set
	logins, groups, events atomic.Int32
	rejectLogin            atomic.Bool
	failEvents             atomic.Bool
	expired                atomic.Bool // the current token no longer works
	lastEvents             atomic.Pointer[http.Request]
}

func (f *fakeSpond) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth2/login" {
		f.logins.Add(1)
		var body struct{ Email, Password string }
		json.NewDecoder(r.Body).Decode(&body)
		if f.rejectLogin.Load() || body.Email != "a@b.no" || body.Password != "hemmelig" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"errorKey": "invalidCredentials"}`))
			return
		}
		f.expired.Store(false)
		w.Write([]byte(`{"accessToken": {"token": "tok", "expiration": "2027-01-01T00:00:00Z"}}`))
		return
	}
	if r.Header.Get("Authorization") != "Bearer tok" || f.expired.Load() {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/groups/":
		f.groups.Add(1)
		w.Write([]byte(groupsJSON))
	case "/sponds/":
		f.events.Add(1)
		f.lastEvents.Store(r)
		if f.failEvents.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if f.eventsBody != "" {
			w.Write([]byte(f.eventsBody))
			return
		}
		w.Write([]byte(eventsJSON))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFake(t *testing.T) (*fakeSpond, *Client) {
	t.Helper()
	f := &fakeSpond{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c := New(Config{Email: "a@b.no", Password: "hemmelig"})
	c.base = srv.URL + "/"
	return f, c
}

// Each event belongs to the child it is on behalf of, with that child's
// reply. Cancelled events and those not for a child stay out.
func TestRefreshReadsTheKidsEvents(t *testing.T) {
	f, c := newFake(t)
	c.refresh(context.Background(), clientNow)

	got := c.eventsAt(clientNow)
	type row struct {
		child, heading string
		reply          Reply
	}
	var rows []row
	for _, e := range got {
		rows = append(rows, row{e.Child, e.Heading, e.Reply})
	}
	want := []row{
		{"Agnes", "TURN 5.-7. klasse", Unanswered},
		{"Jonas", "Heming – Grei", Accepted},
		{"Jonas", "Trening", Declined},
		{"Jonas", "Cup", Waiting},
	}
	if !slices.Equal(rows, want) {
		t.Fatalf("got %+v, want %+v", rows, want)
	}

	match := got[1]
	if match.Location != "Slemdal kunstgress" || match.Description != "Ta med drikke" {
		t.Errorf("place and description: %q, %q", match.Location, match.Description)
	}
	wantStart := time.Date(2026, 11, 7, 10, 0, 0, 0, tz.Oslo)
	if !match.Start.Equal(wantStart) || !match.End.Equal(wantStart.Add(time.Hour)) {
		t.Errorf("times: %v – %v", match.Start, match.End)
	}
	if want := time.Date(2026, 11, 7, 9, 30, 0, 0, tz.Oslo); !match.Meetup.Equal(want) {
		t.Errorf("meetup: %v, want %v", match.Meetup, want)
	}
	if want := time.Date(2026, 11, 6, 21, 0, 0, 0, tz.Oslo); !match.RSVPBy.Equal(want) {
		t.Errorf("deadline: %v, want %v", match.RSVPBy, want)
	}

	q := f.lastEvents.Load().URL.Query()
	if q.Get("minEndTimestamp") != "2026-11-06T11:00:00.000Z" || q.Get("maxEndTimestamp") != "2026-11-20T11:00:00.000Z" {
		t.Errorf("window: %s – %s", q.Get("minEndTimestamp"), q.Get("maxEndTimestamp"))
	}
}

// The groups answer is big and holds everybody's contact details, so it is
// read once, not on every refresh.
func TestGroupsAreReadOnce(t *testing.T) {
	f, c := newFake(t)
	c.refresh(context.Background(), clientNow)
	c.refresh(context.Background(), clientNow.Add(refreshEvery))
	if n := f.groups.Load(); n != 1 {
		t.Errorf("groups read %d times", n)
	}
	if n := f.events.Load(); n != 2 {
		t.Errorf("events read %d times", n)
	}
}

// A member the groups don't know has no name to go by: an empty name would
// match every title in Felles. It is left out, and not looked up every time.
func TestUnknownMemberIsLeftOut(t *testing.T) {
	f, c := newFake(t)
	f.eventsBody = `[{"id": "E1", "heading": "Trening", "behalfOfIds": ["M-GONE"],
		"startTimestamp": "2026-11-08T09:00:00Z", "endTimestamp": "2026-11-08T10:00:00Z", "responses": {}}]`
	c.refresh(context.Background(), clientNow)
	c.refresh(context.Background(), clientNow.Add(refreshEvery))
	if got := c.eventsAt(clientNow.Add(refreshEvery)); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
	if n := f.groups.Load(); n != 1 {
		t.Errorf("groups read %d times", n)
	}
}

func TestExpiredTokenLogsInAgain(t *testing.T) {
	f, c := newFake(t)
	c.refresh(context.Background(), clientNow)
	f.expired.Store(true)
	c.refresh(context.Background(), clientNow.Add(refreshEvery))
	if n := f.logins.Load(); n != 2 {
		t.Errorf("%d logins, want 2", n)
	}
	if n := f.events.Load(); n != 2 { // the refused call doesn't get this far
		t.Errorf("events read %d times, want 2", n)
	}
	if len(c.eventsAt(clientNow.Add(refreshEvery))) != 4 {
		t.Error("no events after logging in again")
	}
}

// Trying a wrong password every few minutes could lock the account, so a
// rejected login isn't tried again before the next restart.
func TestRejectedLoginIsNotRetried(t *testing.T) {
	f, c := newFake(t)
	f.rejectLogin.Store(true)
	c.refresh(context.Background(), clientNow)
	c.refresh(context.Background(), clientNow.Add(refreshEvery))
	if n := f.logins.Load(); n != 1 {
		t.Errorf("%d logins, want 1", n)
	}
	if got := c.eventsAt(clientNow); got != nil {
		t.Errorf("got events: %v", got)
	}
}

// A hiccup keeps what we have. If Spond stays away, the old replies are
// dropped rather than shown as if they were true now.
func TestFailureKeepsEventsForAnHour(t *testing.T) {
	f, c := newFake(t)
	c.refresh(context.Background(), clientNow)
	f.failEvents.Store(true)
	c.refresh(context.Background(), clientNow.Add(refreshEvery))

	if n := len(c.eventsAt(clientNow.Add(59 * time.Minute))); n != 4 {
		t.Errorf("after 59 minutes: %d events, want 4", n)
	}
	if got := c.eventsAt(clientNow.Add(61 * time.Minute)); got != nil {
		t.Errorf("after 61 minutes: %v", got)
	}
}
