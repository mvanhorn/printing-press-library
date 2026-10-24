// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// A privacy label and an IAP list that disappear from a product page show up
// in 'changes' as removals (they used to stay "current" forever).
func TestChangesReportsRemovedIAPListAndPrivacyLabel(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	raw, err := os.ReadFile(filepath.Join("..", "appstore", "testdata", "product_candycrush_us.html"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := appstore.ExtractPageModel(raw)
	if err != nil {
		t.Fatal(err)
	}
	page, err := appstore.ParseProductPage(m)
	if err != nil {
		t.Fatal(err)
	}
	lk := appstore.LookupResult{TrackID: 553834731, TrackName: "Candy Crush Saga", Version: page.Versions[0].Version, Currency: "USD"}
	before, after := s3bTestNow.Add(-72*time.Hour), s3bTestNow.Add(-24*time.Hour)
	page.FetchedAt = before
	if err := store.RecordProductPage(context.Background(), st, "us", page, lk); err != nil {
		t.Fatal(err)
	}
	removed := page
	removed.FetchedAt = after
	removed.InAppPurchases, removed.HasInAppPurchases, removed.IAPNoneShown = []appstore.IAPItem{}, false, true
	removed.Privacy = appstore.Privacy{Types: []appstore.PrivacyType{}, NoneShown: true}
	if err := store.RecordProductPage(context.Background(), st, "us", removed, lk); err != nil {
		t.Fatal(err)
	}

	out, _, err := runS3B(t, db, "changes", "--since", "7d", "--kind", "iap,privacy", "--limit", "0", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	iap, privacy := changeRowsOf(v.Results, "iap"), changeRowsOf(v.Results, "privacy")
	if len(iap) != len(page.InAppPurchases) || len(privacy) != len(page.Privacy.Types) {
		t.Fatalf("want %d IAP and %d privacy removals, got %d and %d:\n%s", len(page.InAppPurchases), len(page.Privacy.Types), len(iap), len(privacy), out)
	}
	for _, r := range append(iap, privacy...) {
		if r.Change != "removed" || r.Date != appstore.FormatTime(after) {
			t.Fatalf("row %+v, want a removal dated %s", r, appstore.FormatTime(after))
		}
	}
}
