// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
)

const occupancyFixture = "testdata/openshelters-occupancy-SYNTHETIC.json"

// stubOccupancy swaps fetchOccupancy for a deterministic function for the test.
func stubOccupancy(t *testing.T, fn func(context.Context) ([]Shelter, error)) {
	t.Helper()
	prev := fetchOccupancy
	t.Cleanup(func() { fetchOccupancy = prev })
	fetchOccupancy = fn
}

// TestParseOccupancy pins the Open_Shelters contract: UPPERCASE attribute keys,
// records tagged "occupancy", live population + breakdown + capacity, the
// trailing-comma city cleaned, state normalized, the pet code mapped onto the
// FEMA vocabulary, the incident carried, and an unreported population staying nil
// (never coerced to 0).
func TestParseOccupancy(t *testing.T) {
	occ, err := parseOccupancy(readFixture(t, occupancyFixture))
	if err != nil {
		t.Fatalf("parseOccupancy: %v", err)
	}
	if len(occ) != 4 {
		t.Fatalf("occupancy fixture: got %d, want 4", len(occ))
	}
	byName := map[string]Shelter{}
	for _, s := range occ {
		if s.Source != "occupancy" {
			t.Errorf("%s: source = %q, want occupancy", s.Name, s.Source)
		}
		byName[s.Name] = s
	}

	// Riverside: trailing-comma city cleaned, lowercase state upcased, population set.
	riv := byName["Riverside Community Center"]
	if riv.City != "Highland" {
		t.Errorf("Riverside city = %q, want Highland (trailing comma stripped)", riv.City)
	}
	if riv.State != "IN" {
		t.Errorf("Riverside state = %q, want IN", riv.State)
	}
	if riv.TotalPopulation == nil || *riv.TotalPopulation != 30 {
		t.Errorf("Riverside total population = %v, want 30", riv.TotalPopulation)
	}
	if riv.IncidentName != "444-26" {
		t.Errorf("Riverside incident = %q, want 444-26", riv.IncidentName)
	}
	if riv.ShelterOpenDate == "" {
		t.Error("Riverside open date should be rendered from epoch millis")
	}

	// Example Civic Arena: full breakdown + capacity + pet mapping + accessibility.
	arena := byName["Example Civic Arena"]
	if arena.TotalPopulation == nil || *arena.TotalPopulation != 50 {
		t.Errorf("Arena total = %v, want 50", arena.TotalPopulation)
	}
	if arena.MedicalNeedsPopulation == nil || *arena.MedicalNeedsPopulation != 5 {
		t.Errorf("Arena medical = %v, want 5", arena.MedicalNeedsPopulation)
	}
	if arena.PetPopulation == nil || *arena.PetPopulation != 3 {
		t.Errorf("Arena pet pop = %v, want 3", arena.PetPopulation)
	}
	if arena.EvacuationCapacity == nil || *arena.EvacuationCapacity != 200 {
		t.Errorf("Arena evac cap = %v, want 200", arena.EvacuationCapacity)
	}
	if !allowsPets(arena.PetAccommodations) {
		t.Errorf("Arena pets = %q, want a pet-friendly code (Co-Located -> COHABIT)", arena.PetAccommodations)
	}
	if !isYes(arena.ADACompliant) || !isYes(arena.WheelchairAccessible) {
		t.Errorf("Arena accessibility not parsed: ada=%q wheelchair=%q", arena.ADACompliant, arena.WheelchairAccessible)
	}

	// Lakeside: an OPEN shelter with population 0 (empty, not unknown).
	lake := byName["Lakeside Open Standby"]
	if lake.TotalPopulation == nil || *lake.TotalPopulation != 0 {
		t.Errorf("Lakeside total = %v, want 0 (open but empty)", lake.TotalPopulation)
	}

	// Unreported: a null population must stay nil, never coerced to 0.
	un := byName["Unreported Population Site"]
	if un.TotalPopulation != nil {
		t.Errorf("Unreported total = %v, want nil (null, not 0)", un.TotalPopulation)
	}
}

// TestParseOccupancyFailsLoud: a valid-JSON-but-wrong-shape body must error, not
// be read as "0 shelters with population".
func TestParseOccupancyFailsLoud(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"features":null}`, `{"status":"blocked"}`} {
		if _, err := parseOccupancy([]byte(body)); err == nil {
			t.Errorf("parseOccupancy(%q) = nil error, want fail-loud", body)
		}
	}
	// An ArcGIS error object must surface as an error.
	if _, err := parseOccupancy([]byte(`{"error":{"code":400,"message":"bad"}}`)); err == nil {
		t.Error("parseOccupancy(error object) = nil error, want surfaced error")
	}
}

// TestOverlayOccupancyFillsPublicOnly locks the privacy contract: a public shelter
// gets its live population and "+occupancy" provenance, but an Open_Shelters row
// with no match in either public feed is WITHHELD, never added, because the CLI
// shows only publicly listed shelters.
func TestOverlayOccupancyFillsPublicOnly(t *testing.T) {
	pop, evac := 42, 300
	base := []Shelter{
		{ShelterID: 1, Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "fema"}, // null population
		{ShelterID: 0, Name: "RC Only Site", State: "WA", Zip: "98001", Source: "redcross"},
	}
	occ := []Shelter{
		{Name: "Lincoln Community Center", State: "IN", Zip: "46322", Source: "occupancy", TotalPopulation: &pop, EvacuationCapacity: &evac}, // matches base #1
		{Name: "Operational Only Shelter", State: "LA", Zip: "70501", Source: "occupancy", TotalPopulation: &pop},                            // non-public: must be withheld
	}
	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if len(out) != 2 {
		t.Fatalf("overlay size = %d, want 2 (fill-only never adds the non-public row)", len(out))
	}
	if filled != 1 || withheld != 1 {
		t.Errorf("filled=%d withheld=%d, want filled=1 withheld=1", filled, withheld)
	}
	if ambiguous != 0 {
		t.Errorf("ambiguous=%d, want 0", ambiguous)
	}
	byName := map[string]Shelter{}
	for _, s := range out {
		byName[s.Name] = s
	}
	if _, present := byName["Operational Only Shelter"]; present {
		t.Error("a shelter present only in the operational roster must NOT be surfaced")
	}
	lincoln := byName["Lincoln Community Center"]
	if lincoln.TotalPopulation == nil || *lincoln.TotalPopulation != 42 {
		t.Errorf("Lincoln population not filled from occupancy: %v", lincoln.TotalPopulation)
	}
	if lincoln.EvacuationCapacity == nil || *lincoln.EvacuationCapacity != 300 {
		t.Errorf("Lincoln capacity not filled from occupancy: %v", lincoln.EvacuationCapacity)
	}
	if lincoln.Source != "fema+occupancy" {
		t.Errorf("Lincoln source = %q, want fema+occupancy", lincoln.Source)
	}
	if lincoln.ShelterID != 1 {
		t.Errorf("Lincoln lost FEMA shelter_id: %d", lincoln.ShelterID)
	}
}

// TestOverlayOccupancyDoubleMatch: two occupancy rows keying the same base row do
// not produce a second row (fill-only); the first fills it, the second is withheld.
func TestOverlayOccupancyDoubleMatch(t *testing.T) {
	p1, p2 := 10, 20
	base := []Shelter{{ShelterID: 1, Name: "Shared Name", State: "TX", City: "Dallas", Address: "1 Main St", Source: "fema"}}
	occ := []Shelter{
		{Name: "Shared Name", State: "TX", City: "Dallas", Address: "1 Main St", Source: "occupancy", TotalPopulation: &p1},
		{Name: "Shared Name", State: "TX", City: "Dallas", Address: "1 Main St", Source: "occupancy", TotalPopulation: &p2},
	}
	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if len(out) != 1 {
		t.Fatalf("double-match = %d rows, want 1 (fill-only adds nothing)", len(out))
	}
	if filled != 1 || withheld != 1 {
		t.Errorf("filled=%d withheld=%d, want filled=1 withheld=1", filled, withheld)
	}
	if ambiguous != 0 {
		t.Errorf("ambiguous=%d, want 0", ambiguous)
	}
}

// TestOverlayOccupancyMissingZIPRequiresAddressCorroboration guards the
// privacy boundary between the public feed and the operational roster. A
// normalized name/state match with a missing public ZIP cannot attach a hidden
// site's population or incident data when the street addresses differ.
func TestOverlayOccupancyMissingZIPRequiresAddressCorroboration(t *testing.T) {
	pop := 25
	base := []Shelter{{
		ShelterID: 1, Name: "Community Center", State: "TX", City: "Dallas",
		Address: "100 Public Way", Source: "fema",
	}}
	occ := []Shelter{{
		Name: "Community Center", State: "TX", City: "Dallas", Zip: "75002",
		Address: "900 Hidden Road", Source: "occupancy", TotalPopulation: &pop,
	}}

	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if filled != 0 || withheld != 1 || ambiguous != 0 {
		t.Fatalf("filled=%d withheld=%d ambiguous=%d, want 0 / 1 / 0", filled, withheld, ambiguous)
	}
	if out[0].TotalPopulation != nil || out[0].Source != "fema" {
		t.Fatalf("uncorroborated occupancy data was attached to public shelter: %+v", out[0])
	}
}

func TestOverlayOccupancyOneMissingZIPWithMatchingStreetAndCity(t *testing.T) {
	pop := 25
	for _, tc := range []struct {
		name      string
		publicZIP string
		opsZIP    string
	}{
		{"public ZIP absent", "", "75002"},
		{"operational ZIP absent", "75002", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := []Shelter{{ShelterID: 1, Name: "Community Center", State: "TX", City: "Dallas", Zip: tc.publicZIP, Address: "100 Main St", Source: "fema"}}
			occ := []Shelter{{Name: "Community Center", State: "TX", City: "Dallas,", Zip: tc.opsZIP, Address: "100 Main St, Dallas", Source: "occupancy", TotalPopulation: &pop}}
			out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
			if filled != 1 || withheld != 0 || ambiguous != 0 || out[0].TotalPopulation == nil || *out[0].TotalPopulation != pop {
				t.Fatalf("one missing ZIP should attach matching site: filled=%d withheld=%d ambiguous=%d row=%+v", filled, withheld, ambiguous, out[0])
			}
		})
	}
}

func TestOverlayOccupancyRejectsCrossCityCollision(t *testing.T) {
	pop := 25
	for _, tc := range []struct {
		name      string
		publicZIP string
		opsZIP    string
	}{
		{"missing ZIP", "", "75002"},
		{"matching ZIP", "75002", "75002"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := []Shelter{{ShelterID: 1, Name: "Community Center", State: "TX", City: "Dallas", Zip: tc.publicZIP, Address: "100 Main St", Source: "fema"}}
			occ := []Shelter{{Name: "Community Center", State: "TX", City: "Austin", Zip: tc.opsZIP, Address: "100 Main St", Source: "occupancy", TotalPopulation: &pop}}
			out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
			if filled != 0 || withheld != 1 || ambiguous != 0 || out[0].TotalPopulation != nil {
				t.Fatalf("cross-city site must stay private: filled=%d withheld=%d ambiguous=%d row=%+v", filled, withheld, ambiguous, out[0])
			}
		})
	}
}

func TestOverlayOccupancyRecognizesSaintCitySpelling(t *testing.T) {
	pop := 25
	base := []Shelter{{ShelterID: 1, Name: "Community Center", State: "MO", City: "Saint Louis", Zip: "63101", Address: "100 Main St", Source: "fema"}}
	occ := []Shelter{{Name: "Community Center", State: "MO", City: "St. Louis,", Zip: "63101", Address: "100 Main St", Source: "occupancy", TotalPopulation: &pop}}
	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if filled != 1 || withheld != 0 || ambiguous != 0 || out[0].TotalPopulation == nil || *out[0].TotalPopulation != pop {
		t.Fatalf("Saint/St. city spelling should match: filled=%d withheld=%d ambiguous=%d row=%+v", filled, withheld, ambiguous, out[0])
	}
}

// TestOverlayOccupancyWithholdsAmbiguousMatch prevents a population from being
// assigned when two public shelters share the same name and state and the
// occupancy row has the same ZIP for both.
func TestOverlayOccupancyWithholdsAmbiguousMatch(t *testing.T) {
	pop := 25
	base := []Shelter{
		{ShelterID: 1, Name: "Shared Name", State: "TX", Zip: "75001", Source: "fema"},
		{ShelterID: 2, Name: "Shared Name", State: "TX", Zip: "75001", Source: "fema"},
	}
	occ := []Shelter{{Name: "Shared Name", State: "TX", Zip: "75001", Source: "occupancy", TotalPopulation: &pop}}

	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if filled != 0 || withheld != 0 || ambiguous != 1 {
		t.Fatalf("filled=%d withheld=%d ambiguous=%d, want 0 / 0 / 1", filled, withheld, ambiguous)
	}
	for _, s := range out {
		if s.TotalPopulation != nil {
			t.Errorf("shelter_id %d received ambiguous population %v", s.ShelterID, *s.TotalPopulation)
		}
	}
}

func TestOverlayOccupancyWithholdsZeroCompatibleCandidates(t *testing.T) {
	pop := 25
	base := []Shelter{
		{ShelterID: 1, Name: "Shared Name", State: "TX", Zip: "75001", Source: "fema"},
		{ShelterID: 2, Name: "Shared Name", State: "TX", Zip: "75002", Source: "fema"},
	}
	occ := []Shelter{{Name: "Shared Name", State: "TX", Zip: "75003", Source: "occupancy", TotalPopulation: &pop}}
	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if filled != 0 || withheld != 1 || ambiguous != 0 {
		t.Fatalf("no compatible candidate: filled=%d withheld=%d ambiguous=%d, want 0/1/0", filled, withheld, ambiguous)
	}
	for _, s := range out {
		if s.TotalPopulation != nil {
			t.Fatalf("unmatched occupancy attached to shelter_id %d", s.ShelterID)
		}
	}
}

// TestOverlayOccupancyPrefersExactZIP verifies an exact ZIP5 resolves a choice
// that would otherwise be ambiguous because another same-name row has no ZIP.
func TestOverlayOccupancyPrefersExactZIP(t *testing.T) {
	pop := 25
	base := []Shelter{
		{ShelterID: 1, Name: "Shared Name", State: "TX", City: "Dallas", Address: "1 Main St", Source: "fema"},
		{ShelterID: 2, Name: "Shared Name", State: "TX", Zip: "75002", Source: "fema"},
	}
	occ := []Shelter{{Name: "Shared Name", State: "TX", City: "Dallas", Zip: "75002-1234", Address: "1 Main St", Source: "occupancy", TotalPopulation: &pop}}

	out, filled, withheld, ambiguous := overlayOccupancy(base, occ)
	if filled != 1 || withheld != 0 || ambiguous != 0 {
		t.Fatalf("filled=%d withheld=%d ambiguous=%d, want 1 / 0 / 0", filled, withheld, ambiguous)
	}
	if out[0].TotalPopulation != nil || out[1].TotalPopulation == nil || *out[1].TotalPopulation != 25 {
		t.Fatalf("exact ZIP occupancy assigned to wrong candidate: %+v", out)
	}
}

// TestFillOccupancyGapFillOnly: occupancy fills nil population but never clobbers
// a value the spine already reported, and descriptive fields only fill gaps.
func TestFillOccupancyGapFillOnly(t *testing.T) {
	existing, incoming := 5, 99
	dst := Shelter{Address: "Spine Address", TotalPopulation: &existing}
	src := Shelter{Address: "Occupancy Address", TotalPopulation: &incoming, EvacuationCapacity: &incoming}
	fillOccupancy(&dst, src)
	if dst.TotalPopulation == nil || *dst.TotalPopulation != 5 {
		t.Errorf("population clobbered: %v, want the spine's 5", dst.TotalPopulation)
	}
	if dst.Address != "Spine Address" {
		t.Errorf("address clobbered: %q, want the spine's", dst.Address)
	}
	if dst.EvacuationCapacity == nil || *dst.EvacuationCapacity != 99 {
		t.Errorf("capacity not gap-filled: %v, want 99", dst.EvacuationCapacity)
	}

	// When the destination is empty/nil, occupancy fills it.
	empty := Shelter{}
	fillOccupancy(&empty, src)
	if empty.TotalPopulation == nil || *empty.TotalPopulation != 99 {
		t.Errorf("gap not filled: %v, want 99", empty.TotalPopulation)
	}
	if empty.Address != "Occupancy Address" {
		t.Errorf("address gap not filled: %q", empty.Address)
	}
}

// TestAddOccupancySource covers the provenance tag composition.
func TestAddOccupancySource(t *testing.T) {
	cases := map[string]string{
		"":               "occupancy",
		"fema":           "fema+occupancy",
		"redcross":       "redcross+occupancy",
		"fema+redcross":  "fema+redcross+occupancy",
		"fema+occupancy": "fema+occupancy", // idempotent
	}
	for in, want := range cases {
		if got := addOccupancySource(in); got != want {
			t.Errorf("addOccupancySource(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestApplyOccupancyOverlay exercises the skip, success, and degrade paths.
func TestApplyOccupancyOverlay(t *testing.T) {
	// Skip paths must not call the network.
	stubOccupancy(t, func(context.Context) ([]Shelter, error) {
		t.Fatal("fetchOccupancy must not be called when occupancy is skipped")
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
			st := applyOccupancyOverlay(context.Background(), tc.flags, feed, tc.spineSource)
			if st.OK || st.Note == "" {
				t.Errorf("%s: state = %+v, want skipped with note", tc.name, st)
			}
			if len(feed.Shelters) != 1 {
				t.Errorf("%s: shelters changed despite skip", tc.name)
			}
		})
	}

	// Success path: occupancy fills the public spine row but withholds the
	// operational-only row and never grows the feed.
	pop := 17
	stubOccupancy(t, func(context.Context) ([]Shelter, error) {
		return []Shelter{
			{Name: "F", State: "IL", Zip: "60601", Source: "occupancy", TotalPopulation: &pop},
			{Name: "Hidden Op Site", State: "IL", Source: "occupancy", TotalPopulation: &pop},
		}, nil
	})
	feed := &shelterFeed{Source: "https://feed", Shelters: []Shelter{{ShelterID: 1, Name: "F", State: "IL", Zip: "60601", Source: "fema"}}}
	st := applyOccupancyOverlay(context.Background(), &rootFlags{dataSource: "auto"}, feed, "live")
	if !st.OK {
		t.Fatalf("success path: state = %+v, want OK", st)
	}
	if len(feed.Shelters) != 1 {
		t.Fatalf("overlay added a non-public shelter: feed size = %d, want 1", len(feed.Shelters))
	}
	if feed.Shelters[0].TotalPopulation == nil || *feed.Shelters[0].TotalPopulation != 17 {
		t.Errorf("occupancy did not fill spine population: %v", feed.Shelters[0].TotalPopulation)
	}
	if !strings.Contains(feed.Shelters[0].Source, "occupancy") {
		t.Errorf("merged row source = %q, want it to record occupancy", feed.Shelters[0].Source)
	}
	if !strings.Contains(st.Note, "Withheld occupancy for 1 operational shelter row") {
		t.Errorf("success note should report the withheld non-public shelter: %q", st.Note)
	}

	// Ambiguity is counted separately from operational-only rows and reported.
	stubOccupancy(t, func(context.Context) ([]Shelter, error) {
		return []Shelter{{Name: "Same", State: "TX", Zip: "75001", TotalPopulation: &pop}}, nil
	})
	feedAmbiguous := &shelterFeed{Shelters: []Shelter{
		{ShelterID: 1, Name: "Same", State: "TX", Zip: "75001", Source: "fema"},
		{ShelterID: 2, Name: "Same", State: "TX", Zip: "75001", Source: "fema"},
	}}
	stAmbiguous := applyOccupancyOverlay(context.Background(), &rootFlags{dataSource: "auto"}, feedAmbiguous, "live")
	if !strings.Contains(stAmbiguous.Note, "Withheld occupancy for 1") {
		t.Errorf("ambiguous note = %q, want separately reported count", stAmbiguous.Note)
	}
	for _, s := range feedAmbiguous.Shelters {
		if s.TotalPopulation != nil {
			t.Errorf("ambiguous occupancy assigned to shelter_id %d", s.ShelterID)
		}
	}

	// Degrade path: a fetch error must not fail the command.
	stubOccupancy(t, func(context.Context) ([]Shelter, error) {
		return nil, context.DeadlineExceeded
	})
	feed2 := &shelterFeed{Source: "https://feed", Shelters: []Shelter{{ShelterID: 1, Source: "fema"}}}
	st2 := applyOccupancyOverlay(context.Background(), &rootFlags{dataSource: "auto"}, feed2, "live")
	if st2.OK || !st2.Attempted || st2.Note == "" {
		t.Errorf("degrade path: state = %+v, want attempted-but-not-OK with a note", st2)
	}
	if len(feed2.Shelters) != 1 {
		t.Errorf("degrade path: shelters changed: %d", len(feed2.Shelters))
	}
}
