package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/kvasbo/tellulf-v9/internal/tz"
)

// A request that never gets an answer must not freeze the refresh loop, which
// waits for it: nothing would update again before a restart.
func TestFetchGivesUpOnAStalledRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	ctx := context.Background()
	svc, err := gcal.NewService(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	fetch := googleFetcher(svc, 50*time.Millisecond)

	done := make(chan error, 1)
	go func() {
		_, err := fetch(ctx, "cal", tz.Now().AddDate(0, 0, 1))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a stalled request should fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the fetch is still waiting for the stalled request")
	}
}
