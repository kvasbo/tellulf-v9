// Package buildinfo holds facts about the binary that are set at build time.
package buildinfo

import "time"

// Time is when the binary was built, as RFC 3339. The Dockerfile sets it with
// -ldflags "-X github.com/kvasbo/tellulf-v9/internal/buildinfo.Time=...";
// it is empty for `go run` and `go build` without the flag.
var Time string

// Built returns the build time, and false when it isn't known.
func Built() (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, Time)
	return t, err == nil
}
