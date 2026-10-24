// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

func TestAppIDForBundleAndAppsWithHistory(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	if id, err := AppIDForBundle(ctx, st, "com.spotify.client"); err != nil || id != "" {
		t.Fatalf("empty store: %q %v", id, err)
	}
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify", BundleID: "com.spotify.client", FetchedAt: t0}
	if err := RecordLookupSnapshot(ctx, st, "us", lk, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"com.spotify.client", "COM.SPOTIFY.CLIENT", " com.spotify.client "} {
		if id, err := AppIDForBundle(ctx, st, in); err != nil || id != "324684580" {
			t.Fatalf("AppIDForBundle(%q) = %q %v", in, id, err)
		}
	}
	if id, _ := AppIDForBundle(ctx, st, "com.example.unknown"); id != "" {
		t.Fatalf("unknown bundle id resolved to %q", id)
	}

	// History beyond listing snapshots: a page-only record (versions, IAP),
	// a privacy label and an event of other apps.
	page := appstore.ProductPage{AdamID: "553834731", InAppPurchases: []appstore.IAPItem{{Position: 1, Name: "Gold Bars", Price: "$0.99"}}, FetchedAt: t0.Add(time.Hour)}
	if err := RecordProductPage(ctx, st, "us", page, appstore.LookupResult{}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordPrivacy(ctx, st, "gb", "111", appstore.Privacy{Types: []appstore.PrivacyType{{Identifier: "DATA_NOT_COLLECTED"}}}, t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := RecordEvents(ctx, st, "us", "222", []appstore.AppEvent{{EventID: "e1", Title: "Event"}}, t0.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := RecordVersions(ctx, st, "333", []appstore.VersionEntry{{Version: "1.0"}}, t0.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	all, err := AppsWithHistory(ctx, st, time.Time{})
	if err != nil || strings.Join(all, ",") != "111,222,324684580,333,553834731" {
		t.Fatalf("AppsWithHistory = %v %v", all, err)
	}
	tracked, _ := TrackedApps(ctx, st, time.Time{})
	if len(tracked) != 1 {
		t.Fatalf("TrackedApps (listing snapshots only) = %+v", tracked)
	}
	recent, err := AppsWithHistory(ctx, st, t0.Add(150*time.Minute))
	if err != nil || strings.Join(recent, ",") != "222,333" {
		t.Fatalf("AppsWithHistory since +2.5h = %v %v", recent, err)
	}
}
