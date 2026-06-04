// Freshness helpers for the local SQLite mirror.
//
// EnsureFresh inspects the most recent sync time for a resource in the local
// store and reports whether it is older than the supplied TTL. Callers
// (typically `gorgias-pp-cli`'s `autoRefreshIfStale` hook) consult this
// before serving a read from local data, and trigger a sync when the data
// is past its useful life.

package cliutil

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// FreshnessChecker is the narrow contract EnsureFresh consults. The
// production implementation is `*store.Store`; tests pass a fake.
type FreshnessChecker interface {
	LastSyncedAt(ctx context.Context, resource string) (time.Time, error)
}

// FreshnessResult captures the outcome of an EnsureFresh check.
type FreshnessResult struct {
	Resource    string        `json:"resource"`
	LastSynced  time.Time     `json:"last_synced"`
	Age         time.Duration `json:"age"`
	TTL         time.Duration `json:"ttl"`
	Stale       bool          `json:"stale"`
	NeverSynced bool          `json:"never_synced"`
}

// ErrNoSyncHistory is returned (wrapped) by LastSyncedAt when a resource
// has never been synced.
var ErrNoSyncHistory = errors.New("resource has never been synced")

// EnsureFresh classifies the freshness of a resource against `ttl`. It does
// NOT mutate state — a caller wanting to refresh stale data must consult
// Stale and trigger sync separately. TTL of zero disables the check.
func EnsureFresh(ctx context.Context, chk FreshnessChecker, resource string, ttl time.Duration) (FreshnessResult, error) {
	if chk == nil {
		return FreshnessResult{Resource: resource}, fmt.Errorf("nil freshness checker")
	}
	res := FreshnessResult{Resource: resource, TTL: ttl}
	last, err := chk.LastSyncedAt(ctx, resource)
	if err != nil {
		if errors.Is(err, ErrNoSyncHistory) || last.IsZero() {
			res.NeverSynced = true
			res.Stale = ttl > 0
			return res, nil
		}
		return res, fmt.Errorf("reading last sync for %s: %w", resource, err)
	}
	res.LastSynced = last
	res.Age = time.Since(last)
	res.Stale = ttl > 0 && res.Age > ttl
	return res, nil
}

// HumanAge returns a compact, human-friendly age (e.g. "12m", "3h", "2d").
func HumanAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
