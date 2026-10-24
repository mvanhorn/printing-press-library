// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// A page+lookup snapshot hashes the description the full listing shows (the
// page's), so an edit the page already shows is visible while a Lookup CDN
// copy still serves the old text.
func TestPageSnapshotHashesThePageDescription(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	lk := appstore.LookupResult{TrackID: 6448311069, TrackName: "Example", Description: "old description", FetchedAt: t0}
	capture := func(desc string, at time.Time) {
		t.Helper()
		page := appstore.ProductPage{AdamID: "6448311069", Description: desc, FetchedAt: at}
		if err := RecordProductPage(ctx, st, "us", page, lk); err != nil {
			t.Fatal(err)
		}
	}
	capture("edited description", t0)
	capture("edited again", t0.Add(time.Hour))

	snaps, err := AppSnapshots(ctx, st, "6448311069", "us", time.Time{})
	if err != nil || len(snaps) != 2 {
		t.Fatalf("snapshots = %d (%v), want 2", len(snaps), err)
	}
	hashes := map[string]bool{}
	for _, s := range snaps {
		hashes[s.Fields.DescriptionHash] = true
	}
	if !hashes[descriptionHash("edited description")] || !hashes[descriptionHash("edited again")] {
		t.Fatalf("description hashes = %v, want the two page descriptions", hashes)
	}
	if snaps[0].ContentHash == snaps[1].ContentHash {
		t.Fatal("a page-only description edit left the content hash unchanged")
	}

	// A page without a description falls back to the Lookup text.
	if err := RecordProductPage(ctx, st, "us", appstore.ProductPage{AdamID: "6448311069", FetchedAt: t0.Add(2 * time.Hour)}, lk); err != nil {
		t.Fatal(err)
	}
	snaps, err = AppSnapshots(ctx, st, "6448311069", "us", t0.Add(2*time.Hour))
	if err != nil || len(snaps) != 1 || snaps[0].Fields.DescriptionHash != descriptionHash("old description") {
		t.Fatalf("fallback snapshot = %+v (%v), want the Lookup description hash", snaps, err)
	}
}
