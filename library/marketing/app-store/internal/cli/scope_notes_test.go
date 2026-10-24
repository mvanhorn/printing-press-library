// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// A table leaves out requested apps and storefronts with no stored data, so
// traction and impact name them in the note, and traction's human table
// prints the same sentence under its rows.
func TestTractionAndImpactNameRequestedScopeWithoutData(t *testing.T) {
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedTractionScenario(t, st, s3bTestNow)
	seedCandyScenario(t, st, s3bTestNow)

	want := "No stored data for app(s) 999999999; storefront(s) fr."
	out, _, err := runS3B(t, db, "traction", s3bSpotify, "999999999", "--countries", "us,fr", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var tv tractionView
	decodeS3B(t, out, &tv)
	if len(tv.Results) == 0 || !strings.Contains(tv.Meta.Note, want) {
		t.Fatalf("traction note = %q (%d rows), want it to contain %q", tv.Meta.Note, len(tv.Results), want)
	}
	out, _, err = runS3B(t, db, "traction", s3bSpotify, "999999999", "--countries", "us,fr", "--human-friendly")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("traction table does not name the scope without data:\n%s", out)
	}

	out, _, err = runS3B(t, db, "impact", s3bCandy, "--at", "latest", "--window", "7d", "--countries", "us,fr", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var iv impactView
	decodeS3B(t, out, &iv)
	if len(iv.Results) == 0 || !strings.Contains(iv.Meta.Note, "No stored data for storefront(s) fr.") {
		t.Fatalf("impact note = %q (%d rows)", iv.Meta.Note, len(iv.Results))
	}
}

// Capture hints name the storefronts that were asked for, not a fixed pair.
func TestCaptureHintsNameTheRequestedStorefronts(t *testing.T) {
	s3bPinClock(t)
	_, db := s3bNewStore(t)

	out, _, err := runS3B(t, db, "traction", s3bSpotify, "--countries", "jp,de", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var tv tractionView
	decodeS3B(t, out, &tv)
	for _, want := range []string{"'ratings 324684580 --countries de,jp'", "'top --genre 36 --country de'", "'reviews 324684580 --countries de,jp'"} {
		if !strings.Contains(tv.Meta.Note, want) {
			t.Fatalf("traction hint = %q, want %s", tv.Meta.Note, want)
		}
	}

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"breakouts", "--countries", "jp,de", "--json"}, "--country de'"},
		{[]string{"breakouts", "--country", "jp", "--json"}, "--country jp'"},
		{[]string{"breakouts", "--json"}, "--country gb'"},
	} {
		out, _, err := runS3B(t, db, tc.args...)
		if err != nil {
			t.Fatal(err)
		}
		var bv breakoutsView
		decodeS3B(t, out, &bv)
		if !strings.Contains(bv.Meta.Note, tc.want) {
			t.Fatalf("%v hint = %q, want %s", tc.args, bv.Meta.Note, tc.want)
		}
	}
}

// changes names never-captured apps instead of counting them as captured, and
// names releases the stored version history dates inside the window that no
// capture pair showed; a release shown as a version row is not repeated.
func TestChangesNoteNamesNeverCapturedAppsAndUnshownReleases(t *testing.T) {
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedSingleCaptureScenario(t, st, s3bTestNow)
	seedCandyScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "changes", "--since", "7d", "--apps", s3bSpotify+",999999999", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	for _, want := range []string{
		"No consecutive captures",
		"each of the 1 captured app(s)",
		"Never captured: 999999999",
		"324684580 9.1.92 (2026-10-19)",
	} {
		if !strings.Contains(v.Meta.Note, want) {
			t.Fatalf("changes note = %q, want it to contain %q", v.Meta.Note, want)
		}
	}
	if strings.Contains(v.Meta.Note, "nothing can have changed") {
		t.Fatalf("changes note still claims nothing changed: %q", v.Meta.Note)
	}

	out, _, err = runS3B(t, db, "changes", "--since", "30d", "--apps", s3bCandy, "--kind", "version", "--json")
	if err != nil {
		t.Fatal(err)
	}
	v = changesView{}
	decodeS3B(t, out, &v)
	shown := changeRowsOf(v.Results, "version")
	if len(shown) == 0 {
		t.Fatalf("candy version rows = %+v", v.Results)
	}
	for _, r := range shown {
		if strings.Contains(v.Meta.Note, s3bCandy+" "+r.To+" (") {
			t.Fatalf("release %s is a version row and is also named in the note %q", r.To, v.Meta.Note)
		}
	}
}
