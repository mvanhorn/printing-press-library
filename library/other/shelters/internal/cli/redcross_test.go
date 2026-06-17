// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const redCrossFixture = "testdata/redcross-ea-view-live-2026-06-17.json"

// stubRedCross swaps fetchRedCross for a deterministic function for the test.
func stubRedCross(t *testing.T, fn func(context.Context) ([]Shelter, error)) {
	t.Helper()
	prev := fetchRedCross
	t.Cleanup(func() { fetchRedCross = prev })
	fetchRedCross = fn
}

// TestParseRedCrossLive pins the real Red Cross contract: 5 open records tagged
// "redcross", coordinates from geometry, blank city filled from the address
// tail, and pet types mapped onto the FEMA vocabulary conservatively.
func TestParseRedCrossLive(t *testing.T) {
	rc, err := parseRedCross(readFixture(t, redCrossFixture))
	if err != nil {
		t.Fatalf("parseRedCross(live): %v", err)
	}
	if len(rc) != 5 {
		t.Fatalf("RC fixture: got %d, want 5", len(rc))
	}
	byName := map[string]Shelter{}
	for _, s := range rc {
		if s.Source != "redcross" {
			t.Errorf("%s: source = %q, want redcross", s.Name, s.Source)
		}
		byName[s.Name] = s
	}
	// Spokane WA is the row FEMA lacks; it must carry coordinates from geometry.
	spk := byName["Spokane Valley United Methodist Church"]
	if spk.State != "WA" || spk.Latitude == nil || spk.Longitude == nil {
		t.Errorf("Spokane row wrong: state=%q coords=%v,%v", spk.State, spk.Latitude, spk.Longitude)
	}
	if spk.Latitude != nil && (*spk.Latitude < 47 || *spk.Latitude > 48) {
		t.Errorf("Spokane latitude = %v, want ~47.6", *spk.Latitude)
	}
	// North High has a blank active_city; it must be filled from the address tail.
	nh := byName["North High School"]
	if nh.City != "Bakersfield" || nh.State != "CA" {
		t.Errorf("North High city/state from address tail = %q/%q, want Bakersfield/CA", nh.City, nh.State)
	}
	// Pet mapping: "Co-Located" -> COHABIT (pet-friendly), "Off-site" -> OFFSITE (not).
	if vv := byName["Valley View High School"]; !allowsPets(vv.PetAccommodations) {
		t.Errorf("Valley View pets = %q, want a pet-friendly code", vv.PetAccommodations)
	}
	if allowsPets(spk.PetAccommodations) {
		t.Errorf("Spokane pets = %q (off-site) must NOT count as on-site pet-friendly", spk.PetAccommodations)
	}
}

func TestParseRedCrossFailsLoud(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"features":null}`, `{"status":"blocked"}`} {
		if _, err := parseRedCross([]byte(raw)); err == nil {
			t.Errorf("parseRedCross(%s) returned no error; want unrecognized-shape error", raw)
		}
	}
	if _, err := parseRedCross([]byte(`{"error":{"code":403,"message":"forbidden"}}`)); err == nil {
		t.Error("parseRedCross(error obj) returned no error")
	}
	rc, err := parseRedCross([]byte(`{"features":[]}`))
	if err != nil || len(rc) != 0 {
		t.Errorf("parseRedCross(empty) = %d,%v; want 0,nil", len(rc), err)
	}
}

func TestRcPetCode(t *testing.T) {
	cases := map[string]string{
		"Co-Located": "COHABIT", "co-located": "COHABIT", "On-site": "ONSITE",
		"Off-site": "OFFSITE", "None": "NONE", "": "", "Pending": "",
	}
	for in, want := range cases {
		if got := rcPetCode(in); got != want {
			t.Errorf("rcPetCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAddressTail(t *testing.T) {
	// Inputs avoid a "<number> <word> St/Ave" street form on purpose: the parser
	// only reads the ", City, ST ZIP" tail, and the library-copy PII scan blocks
	// street addresses, so the test stays valid in both the standalone and library
	// copies.
	c, s, z := parseAddressTail("Rauner Center, Chicago, IL 60612")
	if c != "Chicago" || s != "IL" || z != "60612" {
		t.Errorf("parseAddressTail = %q/%q/%q, want Chicago/IL/60612", c, s, z)
	}
	// ZIP+4 still yields the 5-digit base.
	if _, _, z := parseAddressTail("City Hall, Reno, NV 89501-1234"); z != "89501" {
		t.Errorf("ZIP+4 tail = %q, want 89501", z)
	}
	// A redacted address yields blanks (no false city/state/zip).
	if c, s, z := parseAddressTail("[address omitted in library copy]"); c != "" || s != "" || z != "" {
		t.Errorf("redacted address parsed to %q/%q/%q, want blanks", c, s, z)
	}
}

// TestRedCrossWhereFiltersOpen locks the union to open, publicly-shown rows: the
// status conjunct is what keeps a non-open public Red Cross row from being
// surfaced and labeled "open".
func TestRedCrossWhereFiltersOpen(t *testing.T) {
	if !strings.Contains(redCrossWhere, "active_site_status='Open'") {
		t.Errorf("redCrossWhere must filter to open shelters: %q", redCrossWhere)
	}
	if !strings.Contains(redCrossWhere, "hide_from_public='No'") {
		t.Errorf("redCrossWhere must filter to publicly-shown shelters: %q", redCrossWhere)
	}
}

// TestUnionNoSilentDropOnDoubleMatch: two distinct Red Cross rows that key to the
// same FEMA row must not both be absorbed (which would drop one); the first
// merges, the second is appended.
func TestUnionNoSilentDropOnDoubleMatch(t *testing.T) {
	fema := []Shelter{{ShelterID: 1, Name: "Shelter A", State: "TX", Source: "fema"}}
	rc := []Shelter{
		{Name: "Shelter A", State: "TX", Source: "redcross", RedCrossID: "100"},
		{Name: "Shelter A", State: "TX", Source: "redcross", RedCrossID: "200"},
	}
	out := unionFeeds(fema, rc)
	if len(out) != 2 {
		t.Fatalf("two RC rows matching one FEMA row = %d, want 2 (one merged, one appended; none dropped)", len(out))
	}
}

// TestUnionDedupAndProvenance is the core union test: a shelter present in both
// feeds (with punctuation/spacing/case differences in the name) collapses to one
// "fema+redcross" row keeping the FEMA identity; a FEMA-only and an RC-only row
// both survive; the merged row inherits RC coordinates when FEMA had none.
func TestUnionDedupAndProvenance(t *testing.T) {
	lat, lon := 41.874, -87.681
	fema := []Shelter{
		{ShelterID: 1, Name: "RAUNER CENTER-TRAINING ROOM", State: "IL", Zip: "60612", Source: "fema"}, // null coords
		{ShelterID: 2, Name: "Los Fresnos CISD United Safe Room", State: "TX", Zip: "78566", Source: "fema"},
	}
	rc := []Shelter{
		{Name: "Rauner Center - Training Room", State: "IL", Source: "redcross", Latitude: &lat, Longitude: &lon, RedCrossID: "805294"}, // dup of FEMA #1
		{Name: "Spokane Valley United Methodist Church", State: "WA", Zip: "99206", Source: "redcross"},                                 // RC-only
	}
	out := unionFeeds(fema, rc)
	if len(out) != 3 {
		t.Fatalf("union size = %d, want 3 (1 merged + 1 FEMA-only + 1 RC-only)", len(out))
	}
	byName := map[string]Shelter{}
	for _, s := range out {
		byName[s.Name] = s
	}
	// Rauner collapsed to one fema+redcross row, FEMA identity preserved.
	rauner := byName["RAUNER CENTER-TRAINING ROOM"]
	if rauner.Source != "fema+redcross" {
		t.Errorf("Rauner source = %q, want fema+redcross", rauner.Source)
	}
	if rauner.ShelterID != 1 {
		t.Errorf("Rauner lost FEMA shelter_id: %d", rauner.ShelterID)
	}
	if rauner.RedCrossID != "805294" {
		t.Errorf("Rauner did not capture RC id: %q", rauner.RedCrossID)
	}
	// FEMA had null coords; RC coords must fill in.
	if rauner.Latitude == nil || *rauner.Latitude != lat {
		t.Errorf("Rauner coords not filled from RC: %v", rauner.Latitude)
	}
	// The RC variant name must NOT appear as a separate row.
	if _, dup := byName["Rauner Center - Training Room"]; dup {
		t.Error("RC duplicate of Rauner survived as a separate row")
	}
	// RC-only and FEMA-only both survive with the right tags.
	if byName["Spokane Valley United Methodist Church"].Source != "redcross" {
		t.Error("Spokane (RC-only) missing or mistagged")
	}
	if byName["Los Fresnos CISD United Safe Room"].Source != "fema" {
		t.Error("Los Fresnos (FEMA-only) missing or mistagged")
	}
}

// TestUnionZipDisambiguation: same name+state but different, both-present ZIPs are
// distinct shelters and must NOT be merged.
func TestUnionZipDisambiguation(t *testing.T) {
	fema := []Shelter{{ShelterID: 1, Name: "First Baptist Church", State: "TX", Zip: "75001", Source: "fema"}}
	rc := []Shelter{{Name: "First Baptist Church", State: "TX", Zip: "77002", Source: "redcross"}}
	out := unionFeeds(fema, rc)
	if len(out) != 2 {
		t.Fatalf("different-ZIP same-name = %d rows, want 2 (not merged)", len(out))
	}
	// When a ZIP is missing on one side, they are treated as the same shelter.
	rc2 := []Shelter{{Name: "First Baptist Church", State: "TX", Source: "redcross"}}
	if got := len(unionFeeds(fema, rc2)); got != 1 {
		t.Errorf("missing-ZIP same-name = %d rows, want 1 (merged)", got)
	}
}

// TestApplyRedCrossUnion exercises the skip, success, and degrade paths.
func TestApplyRedCrossUnion(t *testing.T) {
	// Skip paths must not call the network.
	stubRedCross(t, func(context.Context) ([]Shelter, error) {
		t.Fatal("fetchRedCross must not be called when the union is skipped")
		return nil, nil
	})
	for _, tc := range []struct {
		name        string
		flags       *rootFlags
		spineSource string
	}{
		{"no-enrich", &rootFlags{noEnrich: true, dataSource: "auto"}, "live"},
		{"data-source-local", &rootFlags{dataSource: "local"}, "live"},
		{"spine-local", &rootFlags{dataSource: "auto"}, "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feed := &shelterFeed{Source: "u", Shelters: []Shelter{{ShelterID: 1, Source: "fema"}}}
			st := applyRedCrossUnion(context.Background(), tc.flags, feed, tc.spineSource)
			if st.OK || st.Note == "" {
				t.Errorf("%s: state = %+v, want skipped with note", tc.name, st)
			}
			if len(feed.Shelters) != 1 {
				t.Errorf("%s: shelters changed despite skip", tc.name)
			}
		})
	}

	// Success path: RC unions in and the source label reflects it.
	stubRedCross(t, func(context.Context) ([]Shelter, error) {
		return []Shelter{{Name: "RC Only", State: "WA", Source: "redcross"}}, nil
	})
	feed := &shelterFeed{Source: "https://feed", Shelters: []Shelter{{ShelterID: 1, Name: "F", State: "IL", Source: "fema"}}}
	st := applyRedCrossUnion(context.Background(), &rootFlags{dataSource: "auto"}, feed, "live")
	if !st.OK || len(feed.Shelters) != 2 {
		t.Fatalf("success: state=%+v shelters=%d, want ok + 2 shelters", st, len(feed.Shelters))
	}
	if feed.Source == "https://feed" {
		t.Error("source label not updated to reflect the union")
	}

	// Degrade path: a fetch error keeps the FEMA spine, with a note, no error.
	stubRedCross(t, func(context.Context) ([]Shelter, error) {
		return nil, errors.New("403 forbidden")
	})
	feed = &shelterFeed{Source: "https://feed", Shelters: []Shelter{{ShelterID: 1, Source: "fema"}}}
	st = applyRedCrossUnion(context.Background(), &rootFlags{dataSource: "auto"}, feed, "live")
	if st.OK || !st.Attempted || st.Note == "" {
		t.Errorf("degrade: state = %+v, want attempted, not-ok, note", st)
	}
	if len(feed.Shelters) != 1 {
		t.Error("degrade: spine was modified on RC failure")
	}
}

// stubRedCrossHidden swaps fetchRedCrossHidden for a deterministic function.
func stubRedCrossHidden(t *testing.T, fn func(context.Context) ([]Shelter, error)) {
	t.Helper()
	prev := fetchRedCrossHidden
	t.Cleanup(func() { fetchRedCrossHidden = prev })
	fetchRedCrossHidden = fn
}

// TestSuppressHidden locks the privacy contract: a shelter matching a Red-Cross-
// hidden identity (name+state, compatible ZIP) is dropped no matter its source --
// including a FEMA-public shelter the Red Cross keeps off its public map -- while a
// FEMA shelter absent from the hidden set is kept.
func TestSuppressHidden(t *testing.T) {
	shelters := []Shelter{
		{ShelterID: 1, Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "fema+occupancy"}, // FEMA-public but RC-hidden
		{ShelterID: 2, Name: "Family Life Community Church", State: "WA", Zip: "98001", Source: "fema"},       // FEMA-only, not hidden
		{ShelterID: 0, Name: "Open Public RC Site", State: "TX", Zip: "75001", Source: "redcross"},
	}
	hidden := []Shelter{
		{Name: "LINCOLN COMMUNITY CENTER", State: "IN", Zip: "46322", Source: "redcross"}, // case/format differs; must still match
		{Name: "Some Other Hidden Site", State: "FL", Zip: "33101", Source: "redcross"},   // not in the feed; harmless
	}
	kept, removed := suppressHidden(shelters, hidden)
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (only Lincoln matches a hidden identity)", removed)
	}
	names := map[string]bool{}
	for _, s := range kept {
		names[s.Name] = true
	}
	if names["Lincoln Community Center"] {
		t.Error("a Red-Cross-hidden shelter must be suppressed even though FEMA lists it")
	}
	if !names["Family Life Community Church"] || !names["Open Public RC Site"] {
		t.Error("a non-hidden shelter must be kept")
	}

	// An empty hidden set suppresses nothing and returns the input unchanged.
	if k, r := suppressHidden(shelters, nil); r != 0 || len(k) != len(shelters) {
		t.Errorf("empty hidden set: removed=%d kept=%d, want 0 / %d", r, len(k), len(shelters))
	}
}

// TestApplyHiddenSuppression exercises the skip, success, and degrade paths.
func TestApplyHiddenSuppression(t *testing.T) {
	// Skip paths must not call the network.
	stubRedCrossHidden(t, func(context.Context) ([]Shelter, error) {
		t.Fatal("fetchRedCrossHidden must not be called when suppression is skipped")
		return nil, nil
	})
	for _, tc := range []struct {
		name        string
		flags       *rootFlags
		spineSource string
	}{
		{"no-enrich", &rootFlags{noEnrich: true, dataSource: "auto"}, "live"},
		{"data-source-local", &rootFlags{dataSource: "local"}, "live"},
		{"spine-local", &rootFlags{dataSource: "auto"}, "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feed := &shelterFeed{Source: "u", Shelters: []Shelter{{ShelterID: 1, Name: "X", State: "IN", Source: "fema"}}}
			st := applyHiddenSuppression(context.Background(), tc.flags, feed, tc.spineSource)
			if st.OK || st.Note == "" {
				t.Errorf("%s: state = %+v, want skipped with note", tc.name, st)
			}
			if len(feed.Shelters) != 1 {
				t.Errorf("%s: shelters changed despite skip", tc.name)
			}
		})
	}

	// Success path: a FEMA-public-but-RC-hidden shelter is removed.
	stubRedCrossHidden(t, func(context.Context) ([]Shelter, error) {
		return []Shelter{{Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "redcross"}}, nil
	})
	feed := &shelterFeed{Source: "u", Shelters: []Shelter{
		{ShelterID: 1, Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "fema+occupancy"},
		{ShelterID: 2, Name: "Family Life Community Church", State: "WA", Zip: "98001", Source: "fema"},
	}}
	st := applyHiddenSuppression(context.Background(), &rootFlags{dataSource: "auto"}, feed, "live")
	if !st.OK {
		t.Fatalf("success path: state = %+v, want OK", st)
	}
	if len(feed.Shelters) != 1 || feed.Shelters[0].Name != "Family Life Community Church" {
		t.Fatalf("hidden shelter not suppressed: %d shelters remain", len(feed.Shelters))
	}
	if !strings.Contains(st.Note, "Suppressed 1") {
		t.Errorf("note should report the suppression: %q", st.Note)
	}

	// Degrade path: a fetch error must not fail the command and must not suppress.
	stubRedCrossHidden(t, func(context.Context) ([]Shelter, error) {
		return nil, errors.New("403 forbidden")
	})
	feed2 := &shelterFeed{Source: "u", Shelters: []Shelter{{ShelterID: 1, Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "fema"}}}
	st2 := applyHiddenSuppression(context.Background(), &rootFlags{dataSource: "auto"}, feed2, "live")
	if st2.OK || !st2.Attempted || st2.Note == "" {
		t.Errorf("degrade path: state = %+v, want attempted-but-not-OK with a note", st2)
	}
	if len(feed2.Shelters) != 1 {
		t.Errorf("degrade path: shelters changed on fetch failure: %d", len(feed2.Shelters))
	}
}
