package elvia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

func at(h int) time.Time { return time.Date(2026, 11, 26, h, 0, 0, 0, tz.Oslo) }

func value(h int, kwh float64, production bool) string {
	return fmt.Sprintf(`{"startTime":%q,"endTime":%q,"value":%v,"uom":"kWh","production":%v,"verified":false}`,
		at(h).Format(time.RFC3339), at(h+1).Format(time.RFC3339), kwh, production)
}

func meter(values ...string) string {
	series := ""
	for i, v := range values {
		if i > 0 {
			series += ","
		}
		series += v
	}
	return `{"meteringPointId":"1","metervalue":{"resolutionMinutes":60,"timeSeries":[` + series + `]}}`
}

// serve answers the meter values path with the given meteringpoints, after
// checking the request with check.
func serve(t *testing.T, check func(*http.Request), meters ...string) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/customer/metervalues/api/v1/metervalues" {
			http.NotFound(w, r)
			return
		}
		if check != nil {
			check(r)
		}
		body := `{"meteringpoints":[`
		for i, m := range meters {
			if i > 0 {
				body += ","
			}
			body += m
		}
		fmt.Fprint(w, body+"]}")
	}))
	t.Cleanup(srv.Close)
	c := New("tok")
	c.base = srv.URL
	return c
}

func TestHoursParsesTimeSeries(t *testing.T) {
	check := func(r *http.Request) {
		q := r.URL.Query()
		from, _ := time.Parse(time.RFC3339, q.Get("startTime"))
		to, _ := time.Parse(time.RFC3339, q.Get("endTime"))
		if !from.Equal(at(0)) || !to.Equal(at(14)) || q.Get("includeProductionSteps") != "false" {
			t.Errorf("query %v", q)
		}
	}
	c := serve(t, check, meter(value(12, 11.53, false), value(11, 11.436, false)))
	got, err := c.Hours(context.Background(), at(0), at(14))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Start.Equal(at(11)) || got[0].KWh != 11.436 || got[1].KWh != 11.53 {
		t.Errorf("got %+v", got)
	}
}

func TestHoursSkipsProductionAndSumsMeters(t *testing.T) {
	c := serve(t, nil, meter(value(5, 3, false), value(5, 9, true)), meter(value(5, 4, false)))
	got, err := c.Hours(context.Background(), at(0), at(6))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].KWh != 7 {
		t.Errorf("got %+v", got)
	}
}

func TestHoursRejected(t *testing.T) {
	c := serve(t, nil)
	c.token = "wrong"
	if _, err := c.Hours(context.Background(), at(0), at(1)); err == nil {
		t.Error("want an error for a rejected token")
	}
}
