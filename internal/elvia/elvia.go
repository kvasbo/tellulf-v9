// Package elvia reads the home's hourly consumption from Elvia's meter
// values API, with a personal token from Elvia's Min side (Tilganger →
// Opprett token). The data comes per hour, typically less than an hour after
// the hour ends; the latest hours are unverified and may still be adjusted.
package elvia

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// Hour is the energy used in the hour starting at Start.
type Hour struct {
	Start time.Time
	KWh   float64
}

const userAgent = "Tellulf/9.0"

type Client struct {
	http  *http.Client
	base  string
	token string
}

func New(token string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		base:  "https://elvia.azure-api.net",
		token: token,
	}
}

// Hours returns the consumption per hour from from to to, sorted. With more
// than one metering point the hours are summed.
func (c *Client) Hours(ctx context.Context, from, to time.Time) ([]Hour, error) {
	q := url.Values{
		"startTime":              {from.Format(time.RFC3339)},
		"endTime":                {to.Format(time.RFC3339)},
		"includeProductionSteps": {"false"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/customer/metervalues/api/v1/metervalues?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elvia: %s", resp.Status)
	}
	var r struct {
		MeteringPoints []struct {
			MeterValue struct {
				TimeSeries []struct {
					StartTime  time.Time `json:"startTime"`
					Value      float64   `json:"value"`
					Production bool      `json:"production"`
				} `json:"timeSeries"`
			} `json:"metervalue"`
		} `json:"meteringpoints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("elvia: %w", err)
	}
	sum := map[int64]float64{} // by Unix time, as the zone follows DST
	for _, mp := range r.MeteringPoints {
		for _, v := range mp.MeterValue.TimeSeries {
			if !v.Production {
				sum[v.StartTime.Unix()] += v.Value
			}
		}
	}
	out := make([]Hour, 0, len(sum))
	for _, k := range slices.Sorted(maps.Keys(sum)) {
		out = append(out, Hour{Start: time.Unix(k, 0).In(tz.Oslo), KWh: sum[k]})
	}
	return out, nil
}
