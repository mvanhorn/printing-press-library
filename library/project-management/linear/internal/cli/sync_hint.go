// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
	"github.com/spf13/cobra"
)

// defaultMaxAge is the threshold above which a store-backed read is
// considered stale and a sync hint is emitted. Tuned for an active Linear
// workspace where issues churn minutes apart; override per-invocation with
// --max-age (or globally via [cache] max_age in the config file).
const defaultMaxAge = 30 * time.Minute

// hintIfUnsynced writes a one-line "run sync first" hint to stderr when the
// local store has zero rows AND no sync_state row for resourceType. It is
// the cold-start nudge for query commands: instead of silently returning
// `null` or `[]`, the CLI tells the user the store is empty and what to do.
//
// Always check BEFORE returning empty results to keep the hint adjacent to
// the empty output. Writing to stderr keeps --json output a clean pipe.
//
// resourceType is the bare table name (e.g. "issues", "cycles", "projects").
// Returns true when a hint was emitted (so callers can avoid duplicate hints).
func hintIfUnsynced(cmd *cobra.Command, db *store.Store, resourceType string) bool {
	_, lastSynced, count, _ := db.GetSyncState(resourceType)
	if count > 0 || !lastSynced.IsZero() {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  (no %s in local store — run 'linear-pp-cli sync' to populate)\n", resourceType)
	return true
}

// hintIfStale writes a one-line "data is N old" hint to stderr when the
// local store HAS data for resourceType but the last sync is older than
// maxAge. Pairs with hintIfUnsynced (cold-start) — together they cover the
// two empty/old failure modes that drive agents toward stale answers.
//
// Pass flags.maxAge for maxAge; the empty value (0) disables the check.
//
// Returns true when a hint was emitted.
func hintIfStale(cmd *cobra.Command, db *store.Store, resourceType string, maxAge time.Duration) bool {
	if maxAge == 0 {
		return false
	}
	_, lastSynced, count, _ := db.GetSyncState(resourceType)
	if count == 0 || lastSynced.IsZero() {
		return false // hintIfUnsynced's domain
	}
	age := time.Since(lastSynced)
	if age <= maxAge {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  (%s data is %s old, exceeds --max-age=%s — run 'linear-pp-cli sync' to refresh)\n",
		resourceType, age.Round(time.Minute), maxAge)
	return true
}
