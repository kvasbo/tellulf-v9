// Package spotpris fetches the day's spot prices from hvakosterstrommen.no
// (https://www.hvakosterstrommen.no/strompris-api), one hour at a time.
package spotpris

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Price is the spot price for the hour starting at Start, kr/kWh incl. VAT.
type Price struct {
	Start time.Time
	Total float64
}

// ErrNotPublished: the day's prices aren't out yet (tomorrow's come
// around 13:00).
var ErrNotPublished = errors.New("spotpris: prices not published yet")

const userAgent = "Tellulf/9.0"

type Client struct {
	http *http.Client
	base string
	area string // NO1–NO5
}

func New(area string) *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
		base: "https://www.hvakosterstrommen.no/api/v1/prices",
		area: area,
	}
}

// Day returns the prices for the Oslo date of day, sorted, one per hour.
// Shorter slots (quarter hours) are averaged into their hour.
func (c *Client) Day(ctx context.Context, day time.Time) ([]Price, error) {
	day = day.In(tz.Oslo)
	url := fmt.Sprintf("%s/%d/%02d-%02d_%s.json", c.base, day.Year(), day.Month(), day.Day(), c.area)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotPublished
	default:
		return nil, fmt.Errorf("spotpris: %s", resp.Status)
	}
	var entries []struct {
		NOK   float64   `json:"NOK_per_kWh"`
		Start time.Time `json:"time_start"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("spotpris: %w", err)
	}

	vat := 1.25
	if c.area == "NO4" { // Nord-Norge pays no VAT on power
		vat = 1
	}
	type hour struct {
		sum float64
		n   int
	}
	hours := map[int64]*hour{} // by Unix time: the zone in the answer is DST-dependent
	for _, e := range entries {
		k := e.Start.Truncate(time.Hour).Unix() // Oslo's offset is whole hours
		if hours[k] == nil {
			hours[k] = &hour{}
		}
		hours[k].sum += e.NOK
		hours[k].n++
	}
	out := make([]Price, 0, len(hours))
	for _, k := range slices.Sorted(maps.Keys(hours)) {
		h := hours[k]
		out = append(out, Price{Start: time.Unix(k, 0).In(tz.Oslo), Total: h.sum / float64(h.n) * vat})
	}
	return out, nil
}
