package schedule

import (
	"math/rand/v2"
	"time"
)

// Retry delays grow exponentially (with jitter) from BaseBackoff up to
// MaxBackoff while something keeps failing.
const (
	BaseBackoff = 10 * time.Second
	MaxBackoff  = 30 * time.Minute
)

// Backoff returns the delay before attempt n (1-based): half fixed, half
// random, so clients don't retry in lockstep.
func Backoff(attempt int) time.Duration {
	d := BaseBackoff << min(attempt-1, 20)
	d = min(d, MaxBackoff)
	return d/2 + rand.N(d/2)
}
