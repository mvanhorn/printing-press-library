package cliutil

import (
	"context"
	"time"
)

// EnsureFresh runs the supplied refresh closure when `lastSynced` is older
// than `ttl`. Used by auto-refresh to bound how stale the local store can be
// before the next read fans out a sync. Returns the same error the refresh
// function returns; nil when no refresh was needed.
//
// EnsureFresh is intentionally a tiny helper: the real freshness contract
// lives in the per-CLI auto_refresh.go invocations, which compose this with
// per-resource sync calls.
func EnsureFresh(ctx context.Context, lastSynced time.Time, ttl time.Duration, refresh func(context.Context) error) error {
	if refresh == nil {
		return nil
	}
	if !lastSynced.IsZero() && time.Since(lastSynced) < ttl {
		return nil
	}
	return refresh(ctx)
}
