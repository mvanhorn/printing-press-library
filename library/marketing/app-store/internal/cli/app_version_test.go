// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Live on 2026-10-23, `app 324684580 --json` printed version 9.1.92 and
// ratingCount 42,719,156 from a single-id Lookup CDN copy produced at
// 06:35:19Z, while the product page fetched in the same command listed 9.1.94
// (released 17:59:53Z) as newest; the store kept the stale version in the
// page+lookup snapshot, so `search --type apps` and watch-listing showed it.
// Here the fake's Lookup copy is 23 h old (max-age=3600 on a 24 h CDN budget)
// and lists 9.1.90, while the fresh page fixture lists 9.1.92.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestAppTakesTheNewerPagesVersionAndExplainsAnOldLookupCopy(t *testing.T) {
	f := newS2AFake(t)
	for _, r := range f.rows["us"] {
		if fmt.Sprint(r["trackId"]) == "324684580" {
			r["version"] = "9.1.90"
			r["currentVersionReleaseDate"] = "2026-10-08T07:37:17Z"
			r["releaseNotes"] = "Older release notes."
		}
	}
	res := f.mustOK("app", "324684580", "--json")
	v := s2aJSONObject(t, res.stdout)
	if v["version"] != "9.1.92" || v["currentVersionReleaseDate"] != "2026-10-19T13:57:56Z" {
		t.Fatalf("app version = %v released %v, want the newer page's 9.1.92 of 2026-10-19T13:57:56Z", v["version"], v["currentVersionReleaseDate"])
	}
	if notes, _ := v["releaseNotes"].(string); notes == "" || notes == "Older release notes." {
		t.Fatalf("releaseNotes = %q, want the page entry's notes for 9.1.92", notes)
	}
	// The Lookup counts stay the Lookup's, and the output says how old they are.
	if s2aNum(t, v["ratingCount"]) != 42719156 || s2aNum(t, v["userRatingCount"]) != 42719156 {
		t.Fatalf("ratingCount = %v", v["ratingCount"])
	}
	asOf, err := time.Parse(time.RFC3339, fmt.Sprint(v["lookupDataAsOf"]))
	if err != nil || time.Since(asOf) < 22*time.Hour {
		t.Fatalf("lookupDataAsOf = %v (%v), want the 23 h old copy's time", v["lookupDataAsOf"], err)
	}
	notes := fmt.Sprint(v["notes"])
	for _, want := range []string{"version 9.1.92 comes from the product page", "still lists 9.1.90", "come from Apple's CDN copy of", "totalRatings and histogram come from the newer product page"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("notes %s lack %q", notes, want)
		}
	}

	st := f.openStore()
	snaps, err := store.AppSnapshots(context.Background(), st, "324684580", "us", time.Time{})
	if err != nil || len(snaps) != 1 || snaps[0].Version != "9.1.92" || snaps[0].Fields.VersionSource != "page" || snaps[0].Fields.LookupVersion != "9.1.90" {
		t.Fatalf("stored snapshot = %v %+v", err, snaps)
	}
	ratings, err := store.RatingsSeries(context.Background(), st, "324684580", "us", store.SourcePage)
	if err != nil || len(ratings) != 1 || ratings[0].Version != "9.1.92" {
		t.Fatalf("page ratings row = %v %+v, want version 9.1.92", err, ratings)
	}
	// The local app search reads the stored snapshot: it shows the page's version.
	found := s2aJSONArray(t, f.mustOK("search", "spotify", "--type", "apps", "--json").stdout)
	if len(found) == 0 || found[0]["version"] != "9.1.92" {
		t.Fatalf("search --type apps = %v, want version 9.1.92", found)
	}

	// Human output carries the notes too.
	human := f.mustOK("app", "324684580", "--human-friendly")
	if !strings.Contains(human.stdout, "Note:") || !strings.Contains(human.stdout, "9.1.92 comes from the product page") {
		t.Fatalf("human output lacks the notes:\n%s", human.stdout)
	}
}

func TestAppNotesOnlyWhatIsStale(t *testing.T) {
	t.Run("old Lookup copy, same version: only the age note", func(t *testing.T) {
		f := newS2AFake(t)
		v := s2aJSONObject(t, f.mustOK("app", "324684580", "--json").stdout)
		notes := fmt.Sprint(v["notes"])
		if v["version"] != "9.1.92" || strings.Contains(notes, "comes from the product page") || !strings.Contains(notes, "CDN copy of") {
			t.Fatalf("version %v notes %s", v["version"], notes)
		}
	})
	t.Run("fresh Lookup copy: no notes", func(t *testing.T) {
		f := newS2AFake(t)
		f.lookupCacheControl = "max-age=86400"
		res := f.mustOK("app", "324684580", "--json")
		v := s2aJSONObject(t, res.stdout)
		if _, has := v["notes"]; has || v["version"] != "9.1.92" || v["lookupDataAsOf"] == nil {
			t.Fatalf("fresh copy: version %v notes %v lookupDataAsOf %v", v["version"], v["notes"], v["lookupDataAsOf"])
		}
	})
}
