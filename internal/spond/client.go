// Package spond reads the kids' activities from Spond and adds them to the
// calendar snapshot, with whether they have been replied to. It only reads.
//
// Spond has no public API: this is the one its apps use, as reverse-engineered
// by github.com/Olen/Spond, so it may break. All of it lives here, and the rest
// of the code only sees calendar.Calendar.Extend and the Mark and Detail fields
// on calendar.Event, which stay empty without it. To remove it: delete this
// package and the Spond block in main.go, then the Extend hook, the two fields
// and what reads them (grep for Mark and Detail).
package spond

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/calendar"
	"github.com/kvasbo/tellulf-v9/internal/schedule"
	"github.com/kvasbo/tellulf-v9/internal/tz"
)

const (
	apiBase      = "https://api.spond.com/core/v1/"
	refreshEvery = 5 * time.Minute
	// lookAheadDays matches how far the calendar reads.
	lookAheadDays = 14
	// staleAfter: replies older than this are dropped rather than shown as
	// if they were true now, and the screen falls back to the calendars.
	staleAfter = time.Hour
	// timestampLayout is how the API wants its times.
	timestampLayout = "2006-01-02T15:04:05.000Z"
)

type Config struct {
	Email, Password string
}

// Reply is what has been answered for a child.
type Reply int

const (
	Unanswered Reply = iota
	Accepted
	Declined
	Waiting
)

// Event is an activity for one child; an activity for two is two Events.
type Event struct {
	Child       string // first name
	Heading     string
	Description string
	Location    string
	Start, End  time.Time
	Meetup      time.Time // zero without a meeting time
	RSVPBy      time.Time // zero without a deadline
	InviteAt    time.Time // when the invite goes out; zero once it has
	Reply       Reply
}

type Client struct {
	cfg  Config
	base string
	http *http.Client

	// Only touched by the refresh loop.
	token    string
	names    map[string]string // member ID to first name, for the kids
	rejected bool

	mu      sync.RWMutex
	events  []Event
	fetched time.Time
}

func New(cfg Config) *Client {
	return &Client{
		cfg:   cfg,
		base:  apiBase,
		http:  &http.Client{Timeout: 30 * time.Second},
		names: map[string]string{},
	}
}

// Run refreshes the events until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	schedule.Every(ctx, refreshEvery, func(ctx context.Context) { c.refresh(ctx, tz.Now()) })
}

// Extend adds what Spond knows to a calendar snapshot.
func (c *Client) Extend(s calendar.Snapshot) calendar.Snapshot {
	now := tz.Now()
	return merge(s, c.eventsAt(now), now)
}

// eventsAt returns the events, or nil if they are too old to trust.
func (c *Client) eventsAt(now time.Time) []Event {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if now.Sub(c.fetched) > staleAfter {
		return nil
	}
	return c.events
}

func (c *Client) refresh(ctx context.Context, now time.Time) {
	if c.rejected {
		return
	}
	events, err := c.fetchEvents(ctx, now)
	if errors.Is(err, errRejected) {
		// Trying again every few minutes could lock the account.
		c.rejected = true
		slog.Error("spond: login rejected; not trying again before a restart", "err", err)
		return
	}
	if err != nil {
		slog.Error("spond: could not fetch events; keeping previous", "err", err)
		return
	}
	c.mu.Lock()
	c.events, c.fetched = events, now
	c.mu.Unlock()
	slog.Debug("spond: events fetched", "count", len(events))
}

type apiEvent struct {
	Heading         string    `json:"heading"`
	Description     string    `json:"description"`
	StartTimestamp  time.Time `json:"startTimestamp"`
	EndTimestamp    time.Time `json:"endTimestamp"`
	MeetupTimestamp time.Time `json:"meetupTimestamp"`
	RSVPDate        time.Time `json:"rsvpDate"`
	InviteTime      time.Time `json:"inviteTime"`
	Cancelled       bool      `json:"cancelled"`
	BehalfOfIDs     []string  `json:"behalfOfIds"`
	Location        struct {
		Feature string `json:"feature"`
		Address string `json:"address"`
	} `json:"location"`
	Responses struct {
		AcceptedIDs    []string `json:"acceptedIds"`
		DeclinedIDs    []string `json:"declinedIds"`
		WaitinglistIDs []string `json:"waitinglistIds"`
	} `json:"responses"`
}

func (c *Client) fetchEvents(ctx context.Context, now time.Time) ([]Event, error) {
	var raw []apiEvent
	err := c.get(ctx, "sponds/", url.Values{
		"includeComments": {"false"},
		"order":           {"asc"},
		"max":             {"100"},
		"scheduled":       {"true"},
		"minEndTimestamp": {now.UTC().Format(timestampLayout)},
		"maxEndTimestamp": {now.AddDate(0, 0, lookAheadDays).UTC().Format(timestampLayout)},
	}, &raw)
	if err != nil {
		return nil, err
	}
	if err := c.learnNames(ctx, raw); err != nil {
		return nil, err
	}
	var out []Event
	for _, e := range raw {
		if e.Cancelled {
			continue
		}
		for _, id := range e.BehalfOfIDs {
			if name := c.names[id]; name != "" {
				out = append(out, e.forChild(id, name))
			}
		}
	}
	return out, nil
}

func (e apiEvent) forChild(id, name string) Event {
	place := e.Location.Feature
	if place == "" {
		place = e.Location.Address
	}
	return Event{
		Child:       name,
		Heading:     e.Heading,
		Description: e.Description,
		Location:    place,
		Start:       e.StartTimestamp.In(tz.Oslo),
		End:         e.EndTimestamp.In(tz.Oslo),
		Meetup:      e.MeetupTimestamp.In(tz.Oslo),
		RSVPBy:      e.RSVPDate.In(tz.Oslo),
		InviteAt:    e.InviteTime.In(tz.Oslo),
		Reply:       e.reply(id),
	}
}

func (e apiEvent) reply(id string) Reply {
	switch {
	case slices.Contains(e.Responses.AcceptedIDs, id):
		return Accepted
	case slices.Contains(e.Responses.DeclinedIDs, id):
		return Declined
	case slices.Contains(e.Responses.WaitinglistIDs, id):
		return Waiting
	}
	return Unanswered
}

// learnNames looks up the kids' names when an event names someone new. The
// groups answer holds everybody's contact details; only the names of the
// members on our events are kept.
func (c *Client) learnNames(ctx context.Context, events []apiEvent) error {
	wanted := map[string]bool{}
	for _, e := range events {
		for _, id := range e.BehalfOfIDs {
			if _, known := c.names[id]; !known {
				wanted[id] = true
			}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	var groups []struct {
		Members []struct {
			ID        string `json:"id"`
			FirstName string `json:"firstName"`
		} `json:"members"`
	}
	if err := c.get(ctx, "groups/", nil, &groups); err != nil {
		return err
	}
	for id := range wanted {
		c.names[id] = "" // not looked up again if no group has them
	}
	for _, g := range groups {
		for _, m := range g.Members {
			if wanted[m.ID] {
				c.names[m.ID] = m.FirstName
			}
		}
	}
	return nil
}

// get reads a path as JSON, logging in first if needed and once more if the
// token has run out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	if c.token == "" {
		if err := c.login(ctx); err != nil {
			return err
		}
	}
	status, body, err := c.do(ctx, path, query)
	if err == nil && status == http.StatusUnauthorized {
		if err := c.login(ctx); err != nil {
			return err
		}
		status, body, err = c.do(ctx, path, query)
	}
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", path, status)
	}
	return json.Unmarshal(body, out)
}

func (c *Client) do(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+query.Encode(), nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.send(req)
}

func (c *Client) send(req *http.Request) (int, []byte, error) {
	res, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	return res.StatusCode, body, err
}

// errRejected means Spond said no to the credentials, not that it is down.
var errRejected = errors.New("spond: login rejected")

func (c *Client) login(ctx context.Context) error {
	c.token = ""
	payload, err := json.Marshal(map[string]string{"email": c.cfg.Email, "password": c.cfg.Password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"auth2/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	status, body, err := c.send(req)
	if err != nil {
		return err
	}
	if status >= 500 || status == http.StatusTooManyRequests {
		return fmt.Errorf("login: HTTP %d", status)
	}
	var answer struct {
		AccessToken struct {
			Token string `json:"token"`
		} `json:"accessToken"`
		ErrorKey string `json:"errorKey"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.AccessToken.Token == "" {
		// Also what a required second factor looks like.
		return fmt.Errorf("%w: HTTP %d %s", errRejected, status, answer.ErrorKey)
	}
	c.token = answer.AccessToken.Token
	return nil
}
