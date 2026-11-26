// Package tibber reads one home from Tibber: the live feed from its Pulse and
// its hourly consumption. Tellulf uses it for the cabin.
package tibber

import (
	"context"
	"net/http"
	"time"
)

// Hour is the energy used in the hour starting at Start.
type Hour struct {
	Start time.Time
	KWh   float64
}

type Client struct {
	gql    *graphQL
	homeID string
}

func NewClient(token, homeID string) *Client {
	return &Client{
		gql: &graphQL{
			http:     &http.Client{Timeout: 30 * time.Second},
			endpoint: "https://api.tibber.com/v1-beta/gql",
			token:    token,
		},
		homeID: homeID,
	}
}

// RunFeed keeps the live feed connected, passing each measurement to
// onMessage, until ctx is cancelled or Tibber rejects the token.
func (c *Client) RunFeed(ctx context.Context, onMessage func(Measurement)) {
	f := &feed{gql: c.gql, homeID: c.homeID, onMessage: onMessage}
	f.run(ctx)
}

// Hours returns the last hours of consumption that Tibber knows. Hours not
// known yet are left out.
func (c *Client) Hours(ctx context.Context, last int) ([]Hour, error) {
	nodes, err := c.gql.hourlyConsumption(ctx, c.homeID, last)
	if err != nil {
		return nil, err
	}
	var out []Hour
	for _, n := range nodes {
		if n.Consumption != nil {
			out = append(out, Hour{Start: n.From, KWh: *n.Consumption})
		}
	}
	return out, nil
}
