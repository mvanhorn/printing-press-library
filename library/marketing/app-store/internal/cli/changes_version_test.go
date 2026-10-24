// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// A stale Lookup CDN copy captured after a fresh one steps the stored version
// back; that is not a downgrade, and the step forward again is not a second
// release.
func TestChangesIgnoresAStaleVersionStepBack(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	for i, v := range []string{"9.1", "9.2", "9.1", "9.2"} {
		lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify", Version: v, Currency: "USD"}
		if err := store.RecordLookupSnapshot(context.Background(), st, "us", lk, s3bTestNow.Add(time.Duration(i-4)*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	out, _, err := runS3B(t, db, "changes", "--since", "all", "--kind", "version", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	if len(v.Results) != 1 || v.Results[0].From != "9.1" || v.Results[0].To != "9.2" {
		t.Fatalf("want one 9.1 -> 9.2 change, got %s", out)
	}
	if v.Meta.Comparisons != 3 {
		t.Fatalf("comparisons = %d, want 3 consecutive pairs", v.Meta.Comparisons)
	}
}
