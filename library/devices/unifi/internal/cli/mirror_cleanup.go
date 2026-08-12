// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output — regen-merge preserves this file.
// See .printing-press-patches/unifi-mirror-cleanup-on-delete.json for context.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

// cleanupLocalMirrorAfterDelete removes one resource's local mirror row
// after a successful live DELETE, so drift/search/rule-predict/topology
// don't keep showing a resource that no longer exists upstream until the
// next full sync. Best-effort: a missing local mirror (never synced), a
// resource absent from the mirror, or any store error is logged to stderr
// and otherwise ignored — the live delete already succeeded, so a mirror
// cleanup failure must never turn a successful command into a failure.
// Uses a background context, not the calling command's context: cleanup
// runs after the live delete has already succeeded and must not be
// cancelled by the caller's own request timeout.
func cleanupLocalMirrorAfterDelete(resourceType, id, siteID string) {
	db, ok := openWritableStoreForCleanup(resourceType, id)
	if !ok {
		return
	}
	defer db.Close()

	if _, err := db.DeleteResource(resourceType, id, siteID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: local mirror cleanup failed for %s %s: %v\n", resourceType, id, err)
	}
}

// Note: hotspot's bulk "delete-vouchers" endpoint (DELETE
// /v1/sites/{siteId}/hotspot/vouchers?filter=...) is intentionally NOT
// wired to a mirror-cleanup helper here. The API applies an opaque
// server-side filter and does not report which voucher ids it removed, so
// there is no reliable id set to clean up — wiping every locally-mirrored
// hotspot row for the site would be wrong whenever the filter matched only
// some vouchers, trading "stale row visible" for "row wrongly shows as
// gone." Left as a documented gap; run `sync` after a filtered bulk delete
// to refresh the mirror.

// openWritableStoreForCleanup opens the local mirror for writing, or
// returns ok=false when there is nothing to clean up (no local mirror yet)
// or the store could not be opened (logged, non-fatal).
func openWritableStoreForCleanup(resourceType, id string) (*store.Store, bool) {
	dbPath := defaultDBPath("unifi-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, false
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: local mirror cleanup skipped for %s %s, could not open store: %v\n", resourceType, id, err)
		return nil, false
	}
	return db, true
}
