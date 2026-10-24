// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// SQLite's lower() folds ASCII only; listing search matches in any script.
func TestSearchAppsMatchesNonASCIICase(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i, title := range []string{"СберБанк Онлайн", "Über Uns Planer"} {
		lk := appstore.LookupResult{TrackID: int64(1000 + i), TrackName: title, Version: "1.0"}
		if err := RecordLookupSnapshot(ctx, st, "ru", lk, at); err != nil {
			t.Fatal(err)
		}
	}
	for q, want := range map[string]int64{"сбербанк": 1000, "СБЕРБАНК онлайн": 1000, "über": 1001, "ÜBER planer": 1001} {
		got, err := SearchApps(ctx, st, q, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].AppID != appstoreID(want) {
			t.Fatalf("search %q = %+v, want app %d", q, got, want)
		}
	}
}

func appstoreID(n int64) string {
	return appstore.LookupResult{TrackID: n}.AppID()
}
