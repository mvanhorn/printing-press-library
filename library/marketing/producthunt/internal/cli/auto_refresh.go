package cli

import (
	"context"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
)

// defaultFreshnessTTL bounds how stale the local store can be before
// auto-refresh fires for a covered resource. Matches the spec's
// `cache.default_ttl_hours: 24` setting.
const defaultFreshnessTTL = 24 * time.Hour

// autoRefreshIfStale is the freshness hook for read commands wired into the
// machine-owned freshness contract. It checks the last sync timestamp for the
// supplied resource and, when older than `defaultFreshnessTTL`, runs a
// bounded refresh before the read fans out. Honors --data-source: skipped
// entirely under `local` and `live`, only consulted under `auto`.
//
// The refresh closure is supplied by the caller so this helper can be reused
// across resources without coupling the freshness contract to a specific
// sync implementation. Returns nil when no refresh was needed; returns the
// refresh error otherwise — the caller decides whether to surface it as a
// hard fail or a soft warning.
func autoRefreshIfStale(ctx context.Context, dbPath, resource, dataSource string, refresh func(context.Context) error) error {
	if dataSource != "" && dataSource != "auto" {
		return nil
	}
	if refresh == nil {
		return nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil // best-effort: a missing store will be created by the refresh
	}
	defer db.Close()
	_, lastSynced, _, _ := db.GetSyncState(resource)
	return cliutil.EnsureFresh(ctx, lastSynced, defaultFreshnessTTL, refresh)
}
