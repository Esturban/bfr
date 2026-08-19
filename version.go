package main

import "fmt"

// version, commit and date are injected at build time via
// -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
// A `go build ./...` with no ldflags (local dev, `go install`) leaves the
// zero-value defaults below -- still a valid, self-describing answer,
// just not tied to a specific release.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func cmdVersion() {
	fmt.Printf("buf %s (commit %s, built %s)\n", version, commit, date)
}
