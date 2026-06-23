package fares

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

const stalenessBackstop = 35 * 24 * time.Hour

// FreshnessResult reports whether the local fares store is safe to serve.
type FreshnessResult struct {
	OK          bool   // safe to serve fares
	Stale       bool   // store is stale (age backstop exceeded, or a newer feed exists)
	Reason      string // why not OK (set when OK=false)
	PublishDate string // from stored meta, for display
	Warning     string // non-fatal note shown alongside served fares
}

// freshnessProbe returns the remote feed's Last-Modified header. Overridable in tests.
var freshnessProbe = func(ctx context.Context, token, user, pass string) (string, error) {
	if token == "" {
		t, err := Authenticate(ctx, user, pass)
		if err != nil {
			return "", err
		}
		token = t
	}
	return ProbeFreshness(ctx, token)
}

// CheckFreshness decides whether the local fares store is safe to serve fares.
// It returns a non-nil error only for a real ReadMeta DB failure; probe errors
// are handled internally (fall back to the age verdict with a Warning).
func CheckFreshness(ctx context.Context, db *sql.DB, token, user, pass string, offline bool, now time.Time) (FreshnessResult, error) {
	meta, exists, err := ReadMeta(db)
	if err != nil {
		return FreshnessResult{}, fmt.Errorf("fares: CheckFreshness: %w", err)
	}
	if !exists {
		return FreshnessResult{OK: false, Stale: false, Reason: "not synced"}, nil
	}

	result := FreshnessResult{PublishDate: meta.PublishDate}

	// Parse SyncedAt (RFC3339). Treat empty or unparseable as stale.
	if meta.SyncedAt == "" {
		result.Stale = true
		result.Reason = "sync timestamp unreadable"
		return result, nil
	}
	syncedAt, parseErr := time.Parse(time.RFC3339, meta.SyncedAt)
	if parseErr != nil {
		result.Stale = true
		result.Reason = "sync timestamp unreadable"
		return result, nil
	}

	age := now.Sub(syncedAt)

	// Age backstop exceeded: decide without probing.
	if age > stalenessBackstop {
		result.Stale = true
		if offline {
			days := int(age.Hours() / 24)
			result.OK = true
			result.Warning = fmt.Sprintf("serving cached fares %d days old, offline", days)
		} else {
			result.Reason = "last sync exceeds 35-day backstop"
		}
		return result, nil
	}

	// Within backstop.
	if offline {
		result.OK = true
		return result, nil
	}

	// Online: probe the remote feed for a newer Last-Modified.
	remoteLastModified, probeErr := freshnessProbe(ctx, token, user, pass)
	if probeErr != nil {
		// Fall back to the age verdict: OK, but warn.
		result.OK = true
		result.Warning = "freshness could not be confirmed: " + probeErr.Error()
		return result, nil
	}

	// Parse both timestamps and compare. If we cannot prove "newer", treat as not-newer.
	localTime, localErr := http.ParseTime(meta.LastModified)
	remoteTime, remoteErr := http.ParseTime(remoteLastModified)
	if localErr == nil && remoteErr == nil && remoteTime.After(localTime) {
		result.Stale = true
		result.Reason = "a newer fares feed is available"
		return result, nil
	}

	// Equal or older (or unparseable — fail-open: assume not newer).
	result.OK = true
	return result, nil
}
