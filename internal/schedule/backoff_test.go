package schedule

import "testing"

func TestBackoffGrowsAndStaysBounded(t *testing.T) {
	for attempt := 1; attempt < 40; attempt++ {
		d := Backoff(attempt)
		ceiling := min(BaseBackoff<<min(attempt-1, 20), MaxBackoff)
		if d < ceiling/2 || d > ceiling {
			t.Errorf("attempt %d: %v outside [%v, %v]", attempt, d, ceiling/2, ceiling)
		}
	}
}
