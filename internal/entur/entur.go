// Package entur fetches metro departures from Slemdal towards the city.
package entur

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/schedule"
)

// Slemdal station, metro line 1 only. "inbound" = towards the city centre.
const query = `{
	stopPlace(id: "NSR:StopPlace:58268") {
		estimatedCalls(numberOfDepartures: 20, filters: [{select: [{lines: ["RUT:Line:1"]}]}]) {
			realtime
			aimedDepartureTime
			expectedDepartureTime
			destinationDisplay { frontText }
			serviceJourney { directionType }
		}
	}
}`

// A departure counts as delayed when it is expected more than this long
// after the timetabled time.
const delayThreshold = 2 * time.Minute

type Train struct {
	Time        time.Time
	Destination string
	Realtime    bool
	Delayed     bool
}

type response struct {
	Data *struct {
		StopPlace *struct {
			EstimatedCalls []struct {
				Realtime              bool      `json:"realtime"`
				AimedDepartureTime    time.Time `json:"aimedDepartureTime"`
				ExpectedDepartureTime time.Time `json:"expectedDepartureTime"`
				DestinationDisplay    struct {
					FrontText string `json:"frontText"`
				} `json:"destinationDisplay"`
				ServiceJourney struct {
					DirectionType string `json:"directionType"`
				} `json:"serviceJourney"`
			} `json:"estimatedCalls"`
		} `json:"stopPlace"`
	} `json:"data"`
}

type Entur struct {
	client   *http.Client
	endpoint string

	mu     sync.RWMutex
	trains []Train
}

func New() *Entur {
	return &Entur{
		client:   &http.Client{Timeout: 30 * time.Second},
		endpoint: "https://api.entur.io/journey-planner/v3/graphql",
	}
}

// Run refreshes departures every minute until ctx is cancelled.
func (e *Entur) Run(ctx context.Context) {
	schedule.Every(ctx, time.Minute, func(ctx context.Context) {
		if err := e.Update(ctx); err != nil {
			slog.Error("could not update Entur; keeping previous departures", "err", err)
		}
	})
}

// Trains returns upcoming inbound departures, soonest first.
func (e *Entur) Trains() []Train {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return slices.Clone(e.trains)
}

func (e *Entur) Update(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{"query": query})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ET-Client-Name", "kvasbo-tellulf")
	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("entur: %s", res.Status)
	}
	var r response
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return err
	}
	if r.Data == nil || r.Data.StopPlace == nil {
		return errors.New("invalid data from Entur")
	}

	var trains []Train
	for _, c := range r.Data.StopPlace.EstimatedCalls {
		if c.ServiceJourney.DirectionType != "inbound" {
			continue
		}
		trains = append(trains, Train{
			Time:        c.ExpectedDepartureTime,
			Destination: c.DestinationDisplay.FrontText,
			Realtime:    c.Realtime,
			Delayed:     c.ExpectedDepartureTime.Sub(c.AimedDepartureTime) > delayThreshold,
		})
	}
	slices.SortStableFunc(trains, func(a, b Train) int { return a.Time.Compare(b.Time) })

	e.mu.Lock()
	e.trains = trains
	e.mu.Unlock()
	slog.Info("Entur updated", "trains", len(trains))
	return nil
}
