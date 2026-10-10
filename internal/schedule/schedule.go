// Package schedule has the two timing helpers every data source needs. Both
// stop cleanly when the context is cancelled.
package schedule

import (
	"context"
	"time"
)

// Sleep waits for d, returning false early if ctx is cancelled.
func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Every runs fn immediately and then every interval until ctx is cancelled.
// Runs never overlap: a slow fn delays the next tick instead of piling up.
func Every(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
