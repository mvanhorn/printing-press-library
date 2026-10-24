// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestWatchListingHelpWires(t *testing.T) {
	r := s3aRun(t, "watch-listing", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "watch-listing <app> [flags]") {
		t.Fatalf("watch-listing --help: exit %d\n%s", r.code, r.stdout)
	}
}

// watchListingJSON is the decoded view with raw change values.
type watchListingJSON struct {
	AppID      string           `json:"appId"`
	Storefront string           `json:"storefront"`
	Source     string           `json:"source"`
	Snapshots  int              `json:"snapshots"`
	Previous   *watchListingRef `json:"previous"`
	Latest     *watchListingRef `json:"latest"`
	Changed    bool             `json:"changed"`
	Changes    []struct {
		Field  string          `json:"field"`
		From   json.RawMessage `json:"from"`
		To     json.RawMessage `json:"to"`
		Detail json.RawMessage `json:"detail"`
	} `json:"changes"`
	Note string `json:"note"`
}

func TestWatchListingVersionChange(t *testing.T) {
	db := s3aSeededDB(t)
	var v watchListingJSON
	s3aDecode(t, s3aRun(t, "watch-listing", "324684580", "--db", db, "--country", "us", "--json"), &v)
	if !v.Changed || v.Source != store.SourcePageLookup || v.Snapshots != 3 {
		t.Fatalf("view = %+v", v)
	}
	if !v.Previous.CapturedAt.Equal(s3aT0.Add(24*time.Hour)) || !v.Latest.CapturedAt.Equal(s3aT0.Add(48*time.Hour)) {
		t.Fatalf("compared %v -> %v, want the two latest snapshots", v.Previous.CapturedAt, v.Latest.CapturedAt)
	}
	fields := []string{}
	byField := map[string]int{}
	for i, c := range v.Changes {
		fields = append(fields, c.Field)
		byField[c.Field] = i
	}
	if strings.Join(fields, ",") != "version,inAppPurchases,events" {
		t.Fatalf("changed fields = %v, want version,inAppPurchases,events", fields)
	}
	ver := v.Changes[byField["version"]]
	if s3aCompactJSON(ver.From) != `"9.1.90"` || s3aCompactJSON(ver.To) != `"9.1.92"` || !strings.Contains(s3aCompactJSON(ver.Detail), "smarter shuffle") {
		t.Fatalf("version change = %s -> %s (%s)", ver.From, ver.To, ver.Detail)
	}
	iap := v.Changes[byField["inAppPurchases"]]
	if !strings.Contains(s3aCompactJSON(iap.Detail), `"repriced":[{"name":"Premium Individual","from":"$11.99","to":"$12.99"}]`) {
		t.Fatalf("IAP detail = %s", iap.Detail)
	}
	ev := v.Changes[byField["events"]]
	if !strings.Contains(s3aCompactJSON(ev.Detail), `"eventId":"6740000001","title":"Sample Live Session"`) || s3aCompactJSON(ev.From) != "[]" {
		t.Fatalf("events change = %s -> %s (%s)", ev.From, ev.To, ev.Detail)
	}
}

func TestWatchListingEqualHashesReportNothing(t *testing.T) {
	db := s3aSeededDB(t)
	var v watchListingJSON
	s3aDecode(t, s3aRun(t, "watch-listing", "553834731", "--db", db, "--json"), &v)
	if v.Changed || len(v.Changes) != 0 || v.Previous == nil || v.Previous.ContentHash != v.Latest.ContentHash {
		t.Fatalf("equal hashes must report no change: %+v", v)
	}
	if !strings.Contains(v.Note, "same content hash") {
		t.Fatalf("note = %q", v.Note)
	}
	// Spotify's first two snapshots are identical too: the pure diff agrees.
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	snaps, _ := store.AppSnapshots(context.Background(), st, "324684580", "us", time.Time{})
	if got := s3aDiffListing(snaps[0], snaps[1], nil); len(got) != 0 {
		t.Fatalf("diff of equal snapshots = %+v", got)
	}
	// Equal hashes win even if a non-hashed field (ratings) differs.
	a, b := snaps[0], snaps[1]
	b.RatingCount, b.AverageRating = a.RatingCount+500, 1.0
	if got := s3aDiffListing(a, b, nil); len(got) != 0 {
		t.Fatalf("rating drift must not be a listing change: %+v", got)
	}
}

func TestWatchListingSingleSnapshotAndOtherStorefront(t *testing.T) {
	db := s3aSeededDB(t)
	var v watchListingJSON
	s3aDecode(t, s3aRun(t, "watch-listing", "6448311069", "--db", db, "--json"), &v)
	if v.Previous != nil || v.Latest == nil || len(v.Changes) != 0 || v.Source != store.SourceLookup {
		t.Fatalf("single snapshot = %+v", v)
	}
	if !strings.Contains(v.Note, "only one lookup snapshot") || !strings.Contains(v.Note, "needs two") {
		t.Fatalf("note = %q", v.Note)
	}
	v = watchListingJSON{}
	s3aDecode(t, s3aRun(t, "watch-listing", "1482155847", "--db", db, "--country", "us", "--json"), &v)
	if v.Latest != nil || len(v.Changes) != 0 || !strings.Contains(v.Note, "snapshots exist in gb (1)") {
		t.Fatalf("other storefront = %+v", v)
	}
	// A store URL picks its storefront when --country is not given.
	v = watchListingJSON{}
	s3aDecode(t, s3aRun(t, "watch-listing", "https://apps.apple.com/gb/app/royal-match/id1482155847", "--db", db, "--json"), &v)
	if v.Storefront != "gb" || v.Latest == nil || v.Snapshots != 1 {
		t.Fatalf("URL storefront = %+v", v)
	}
}

func TestWatchListingPrefersFullSnapshots(t *testing.T) {
	db := s3aSeededDB(t)
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	// Two later lookup-only snapshots (prices/compare) must not displace the full pair.
	for i := 0; i < 2; i++ {
		l := s3aListings()[2]
		l.lookupOnly, l.at = true, s3aT0.Add(time.Duration(72+i)*time.Hour)
		if err := s3aRecordListing(context.Background(), st, l); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Close()
	var v watchListingJSON
	s3aDecode(t, s3aRun(t, "watch-listing", "324684580", "--db", db, "--json"), &v)
	if v.Source != store.SourcePageLookup || len(v.Changes) != 3 {
		t.Fatalf("auto source = %s with %d changes", v.Source, len(v.Changes))
	}
	v = watchListingJSON{}
	s3aDecode(t, s3aRun(t, "watch-listing", "324684580", "--db", db, "--source", "lookup", "--json"), &v)
	if v.Source != store.SourceLookup || v.Changed || v.Snapshots != 2 {
		t.Fatalf("pinned lookup = %+v", v)
	}
}

func TestWatchListingEmptyAndErrors(t *testing.T) {
	r := s3aRun(t, "watch-listing", "324684580", "--db", t.TempDir()+"/absent.db", "--json")
	var v watchListingJSON
	s3aDecode(t, r, &v)
	if len(v.Changes) != 0 || v.Latest != nil || !strings.Contains(v.Note, "no local listing snapshots") || !strings.Contains(r.stderr, "no local mirror at") {
		t.Fatalf("missing mirror = %+v / %q", v, r.stderr)
	}
	if !strings.Contains(r.stdout, `"changes": []`) {
		t.Fatalf("changes must be [] not null:\n%s", r.stdout)
	}
	v = watchListingJSON{}
	s3aDecode(t, s3aRun(t, "watch-listing", "324684580", "--db", s3aEmptyDB(t), "--json"), &v)
	if len(v.Changes) != 0 || !strings.Contains(v.Note, "no stored snapshots of app 324684580 in us") {
		t.Fatalf("empty store = %+v", v)
	}
	for _, args := range [][]string{
		{"watch-listing", "--json"},
		{"watch-listing", "__printing_press_invalid__"},
		{"watch-listing", "324684580", "--source", "cache"},
		{"watch-listing", "1", "2"},
	} {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
}

// s3aCompactJSON renders a raw JSON value without whitespace.
func s3aCompactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}
