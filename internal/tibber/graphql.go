package tibber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Tibber requires a User-Agent naming both platform and driver version.
const userAgent = "Tellulf/9.0 tellulf-tibber-go/1.0"

// errUnauthorized means Tibber rejected the token. Per Tibber's client
// requirements we must not keep retrying with it.
var errUnauthorized = errors.New("tibber rejected the API token")

type graphQL struct {
	http     *http.Client
	endpoint string
	token    string
}

type gqlError struct {
	Message string `json:"message"`
}

// query POSTs a GraphQL query and decodes its data into out.
func (g *graphQL) query(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return errUnauthorized
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("tibber: %s", res.Status)
	}

	var r struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return err
	}
	if len(r.Errors) > 0 {
		msgs := make([]string, len(r.Errors))
		for i, e := range r.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("tibber: %s", strings.Join(msgs, "; "))
	}
	return json.Unmarshal(r.Data, out)
}

// --- Queries ------------------------------------------------------------------

type pricePoint struct {
	Start time.Time
	Total float64 // kr/kWh incl. VAT
}

// Prices for today and tomorrow. Tibber asks clients to fetch these once a
// day and cache them instead of polling the current price.
const pricesQuery = `query($homeId: ID!) {
  viewer { home(id: $homeId) { currentSubscription { priceInfo(resolution: HOURLY) {
    today { total startsAt }
    tomorrow { total startsAt }
  } } } }
}`

func (g *graphQL) prices(ctx context.Context, homeID string) ([]pricePoint, error) {
	type price struct {
		Total    *float64  `json:"total"`
		StartsAt time.Time `json:"startsAt"`
	}
	var r struct {
		Viewer struct {
			Home struct {
				CurrentSubscription *struct {
					PriceInfo *struct {
						Today    []price `json:"today"`
						Tomorrow []price `json:"tomorrow"`
					} `json:"priceInfo"`
				} `json:"currentSubscription"`
			} `json:"home"`
		} `json:"viewer"`
	}
	if err := g.query(ctx, pricesQuery, map[string]any{"homeId": homeID}, &r); err != nil {
		return nil, err
	}
	sub := r.Viewer.Home.CurrentSubscription
	if sub == nil || sub.PriceInfo == nil {
		return nil, errors.New("tibber: home has no price info")
	}
	var out []pricePoint
	for _, p := range append(sub.PriceInfo.Today, sub.PriceInfo.Tomorrow...) {
		if p.Total != nil {
			out = append(out, pricePoint{Start: p.StartsAt, Total: *p.Total})
		}
	}
	return out, nil
}

const consumptionQuery = `query($homeId: ID!, $last: Int!) {
  viewer { home(id: $homeId) { consumption(resolution: HOURLY, last: $last) {
    nodes { from consumption cost }
  } } }
}`

// Tibber caps hourly consumption queries at 744 nodes (31 days).
const maxConsumptionNodes = 744

type consumptionNode struct {
	From        time.Time `json:"from"`
	Consumption *float64  `json:"consumption"` // kWh, null when not yet known
	Cost        *float64  `json:"cost"`
}

func (g *graphQL) hourlyConsumption(ctx context.Context, homeID string, last int) ([]consumptionNode, error) {
	var r struct {
		Viewer struct {
			Home struct {
				Consumption *struct {
					Nodes []consumptionNode `json:"nodes"`
				} `json:"consumption"`
			} `json:"home"`
		} `json:"viewer"`
	}
	vars := map[string]any{"homeId": homeID, "last": min(last, maxConsumptionNodes)}
	if err := g.query(ctx, consumptionQuery, vars, &r); err != nil {
		return nil, err
	}
	if r.Viewer.Home.Consumption == nil {
		return nil, nil
	}
	return r.Viewer.Home.Consumption.Nodes, nil
}

// The websocket URL must be looked up dynamically, and Tibber asks clients to
// check that the home still has a real-time device before (re)connecting.
const feedInfoQuery = `query($homeId: ID!) {
  viewer {
    websocketSubscriptionUrl
    home(id: $homeId) { features { realTimeConsumptionEnabled } }
  }
}`

func (g *graphQL) feedInfo(ctx context.Context, homeID string) (url string, realTime bool, err error) {
	var r struct {
		Viewer struct {
			WebsocketSubscriptionURL string `json:"websocketSubscriptionUrl"`
			Home                     struct {
				Features *struct {
					RealTimeConsumptionEnabled *bool `json:"realTimeConsumptionEnabled"`
				} `json:"features"`
			} `json:"home"`
		} `json:"viewer"`
	}
	if err := g.query(ctx, feedInfoQuery, map[string]any{"homeId": homeID}, &r); err != nil {
		return "", false, err
	}
	f := r.Viewer.Home.Features
	realTime = f != nil && f.RealTimeConsumptionEnabled != nil && *f.RealTimeConsumptionEnabled
	if r.Viewer.WebsocketSubscriptionURL == "" {
		return "", realTime, errors.New("tibber: no websocket subscription url")
	}
	return r.Viewer.WebsocketSubscriptionURL, realTime, nil
}
