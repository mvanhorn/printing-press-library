package cliutil

import (
	"os"
	"strings"
	"time"
)

// DefaultStaleAfter matches the threshold doctor's cache report uses. Keeping
// the two in step matters: doctor is where a user reads "your mirror is
// stale", and a refresh gate that disagreed with that reading would make the
// diagnosis untrustworthy.
const DefaultStaleAfter = 6 * time.Hour

// FreshnessVerdict explains a staleness decision. The reason travels with the
// verdict so callers can tell a user *why* nothing refreshed, which is the
// difference between a silent no-op and an actionable message.
type FreshnessVerdict struct {
	Stale  bool
	Age    time.Duration
	Reason string
}

// EnsureFresh decides whether data last written at lastSynced is stale enough
// to warrant a refresh. A zero lastSynced means the resource has never
// completed a sync; that is reported as not-stale with an explicit reason,
// because there is no prior successful run to reproduce and re-running one
// unattended would be a guess rather than a refresh.
func EnsureFresh(lastSynced time.Time, staleAfter time.Duration) FreshnessVerdict {
	if staleAfter <= 0 {
		staleAfter = DefaultStaleAfter
	}
	if lastSynced.IsZero() {
		return FreshnessVerdict{Reason: "never synced"}
	}
	age := time.Since(lastSynced)
	if age <= staleAfter {
		return FreshnessVerdict{Age: age, Reason: "fresh"}
	}
	return FreshnessVerdict{Stale: true, Age: age, Reason: "older than " + staleAfter.String()}
}

// StaleAfterFromEnv reads FOODPANDA_STALE_AFTER as a Go duration. An
// unparseable value falls back to the default rather than erroring: a typo in
// an environment variable should not make every command fail.
func StaleAfterFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FOODPANDA_STALE_AFTER"))
	if raw == "" {
		return DefaultStaleAfter
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return DefaultStaleAfter
	}
	return d
}

// AutoRefreshEnabled reports whether unattended refresh is switched on.
// Refresh is opt-in by design: these commands read a local mirror, and a user
// who runs one does not expect it to reach the network. Anything that makes a
// read command hit the network has to be asked for.
func AutoRefreshEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FOODPANDA_AUTO_REFRESH"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
