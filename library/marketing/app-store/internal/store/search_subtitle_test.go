// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// A subtitle the developer removed (the latest page capture shows none) must
// not be searched or returned.
func TestSearchAppsRemovedSubtitleStaysRemoved(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	lk := appstore.LookupResult{TrackID: 1000, TrackName: "Plain Title", Version: "1.0"}
	lk.FetchedAt = t0
	if err := RecordProductPage(ctx, st, "us", appstore.ProductPage{AdamID: "1000", Subtitle: "Old tagline", FetchedAt: t0}, lk); err != nil {
		t.Fatal(err)
	}
	lk.FetchedAt = t0.Add(48 * time.Hour)
	if err := RecordProductPage(ctx, st, "us", appstore.ProductPage{AdamID: "1000", FetchedAt: t0.Add(48 * time.Hour)}, lk); err != nil {
		t.Fatal(err)
	}
	if got, err := SearchApps(ctx, st, "tagline", "", 10); err != nil || len(got) != 0 {
		t.Fatalf("a removed subtitle still matched: %+v %v", got, err)
	}
	// A later lookup-only capture borrows the latest page capture's subtitle.
	if err := RecordProductPage(ctx, st, "us", appstore.ProductPage{AdamID: "1000", Subtitle: "New tagline", FetchedAt: t0.Add(72 * time.Hour)}, lk); err != nil {
		t.Fatal(err)
	}
	if err := RecordLookupSnapshot(ctx, st, "us", appstore.LookupResult{TrackID: 1000, TrackName: "Plain Title", Version: "1.0"}, t0.Add(96*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, err := SearchApps(ctx, st, "new tagline", "", 10); err != nil || len(got) != 1 || got[0].Subtitle != "New tagline" {
		t.Fatalf("lookup-only latest snapshot: %+v %v", got, err)
	}
}
