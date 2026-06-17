// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	enrichLiveFixture      = "testdata/femanss-live-2026-06-16.json"
	enrichSyntheticFixture = "testdata/femanss-active-SYNTHETIC.json"
)

// stubEnrichment swaps fetchEnrichment for a deterministic function for the test
// and restores it afterward.
func stubEnrichment(t *testing.T, fn func(context.Context) (map[int]Shelter, error)) {
	t.Helper()
	prev := fetchEnrichment
	t.Cleanup(func() { fetchEnrichment = prev })
	fetchEnrichment = fn
}

// TestAttrEpochDateStr pins the UTC date conversion against a concrete oracle:
// 1781222400000 ms is exactly midnight UTC 2026-06-12 (86400000*20616). The
// .UTC() in the impl must keep this from shifting a day on a Pacific machine.
func TestAttrEpochDateStr(t *testing.T) {
	m := map[string]any{"d": float64(1781222400000), "missing": nil, "str": "x"}
	if got := attrEpochDateStr(m, "d"); got != "2026-06-12" {
		t.Errorf("attrEpochDateStr(1781222400000) = %q, want 2026-06-12", got)
	}
	if got := attrEpochDateStr(m, "missing"); got != "" {
		t.Errorf("attrEpochDateStr(null) = %q, want empty", got)
	}
	if got := attrEpochDateStr(m, "absent"); got != "" {
		t.Errorf("attrEpochDateStr(absent) = %q, want empty", got)
	}
	if got := attrEpochDateStr(m, "str"); got != "" {
		t.Errorf("attrEpochDateStr(non-number) = %q, want empty", got)
	}
}

// TestParseEnrichmentLiveSparse pins the REAL enrichment contract: 6 records
// keyed by shelter_id, county populated, the coded flags UNK, populations null,
// and the open date converted from epoch millis.
func TestParseEnrichmentLiveSparse(t *testing.T) {
	em, err := parseEnrichment(readFixture(t, enrichLiveFixture))
	if err != nil {
		t.Fatalf("parseEnrichment(live): %v", err)
	}
	if len(em) != 6 {
		t.Fatalf("live enrichment: got %d records, want 6", len(em))
	}
	s, ok := em[368133]
	if !ok {
		t.Fatal("expected shelter_id 368133 in live enrichment")
	}
	if s.CountyParish != "Lake" {
		t.Errorf("368133 county = %q, want Lake", s.CountyParish)
	}
	if s.FacilityType != "SHELTER" {
		t.Errorf("368133 facility_type = %q, want SHELTER", s.FacilityType)
	}
	if s.IncidentName != "IndianaRegion-IN_TOR" {
		t.Errorf("368133 incident = %q, want IndianaRegion-IN_TOR", s.IncidentName)
	}
	if s.ShelterOpenDate != "2026-06-12" {
		t.Errorf("368133 opened = %q, want 2026-06-12", s.ShelterOpenDate)
	}
	if s.GeneratorOnsite != "UNK" {
		t.Errorf("368133 generator = %q, want UNK (genuinely unreported)", s.GeneratorOnsite)
	}
	if s.GeneralPopulation != nil || s.PetPopulation != nil {
		t.Errorf("368133 populations should be nil in the sparse capture")
	}
}

// TestParseEnrichmentSynthetic confirms the populated synthetic enrichment parses
// with the right values and coded normalization.
func TestParseEnrichmentSynthetic(t *testing.T) {
	em, err := parseEnrichment(readFixture(t, enrichSyntheticFixture))
	if err != nil {
		t.Fatalf("parseEnrichment(synthetic): %v", err)
	}
	if len(em) != 12 {
		t.Fatalf("synthetic enrichment: got %d records, want 12", len(em))
	}
	s := em[500101]
	if s.CountyParish != "Harris" {
		t.Errorf("500101 county = %q, want Harris", s.CountyParish)
	}
	if s.GeneratorOnsite != "YES" {
		t.Errorf("500101 generator = %q, want YES", s.GeneratorOnsite)
	}
	if s.IncidentName != "Hurricane EXERCISE-2026" {
		t.Errorf("500101 incident = %q", s.IncidentName)
	}
	if s.ShelterOpenDate != "2026-06-11" {
		t.Errorf("500101 opened = %q, want 2026-06-11", s.ShelterOpenDate)
	}
	if s.GeneralPopulation == nil || *s.GeneralPopulation != 100 {
		t.Errorf("500101 general_population = %v, want 100", s.GeneralPopulation)
	}
	if s.OrgMainPhone != "713-555-0101" {
		t.Errorf("500101 org_main_phone = %q, want reserved 713-555-0101", s.OrgMainPhone)
	}
	// Null-population shelter keeps nil breakdown (never fabricated 0).
	if p := em[500110]; p.GeneralPopulation != nil {
		t.Errorf("500110 general_population = %v, want nil", p.GeneralPopulation)
	}
	// Second incident exists so brief by_incident has >1 bucket.
	if em[500106].IncidentName != "EX-Inland-Flood-2026" {
		t.Errorf("500106 incident = %q, want EX-Inland-Flood-2026", em[500106].IncidentName)
	}
}

// TestMergeEnrichmentJoinsByID verifies the spine keeps its core fields while the
// enrichment fields land on the matching shelter_id, and that a non-matching
// spine row and a zero-id row are left untouched.
func TestMergeEnrichmentJoinsByID(t *testing.T) {
	spine := parseFixture(t, syntheticFixture)
	em, err := parseEnrichment(readFixture(t, enrichSyntheticFixture))
	if err != nil {
		t.Fatalf("parseEnrichment: %v", err)
	}
	mergeEnrichment(spine, em)

	var s500101 *Shelter
	for i := range spine {
		if spine[i].ShelterID == 500101 {
			s500101 = &spine[i]
		}
	}
	if s500101 == nil {
		t.Fatal("500101 missing from spine")
	}
	// Spine core field preserved.
	if !strings.Contains(s500101.Name, "Bayou Civic Center") {
		t.Errorf("spine name clobbered: %q", s500101.Name)
	}
	// Enrichment merged in.
	if s500101.CountyParish != "Harris" {
		t.Errorf("merge: 500101 county = %q, want Harris", s500101.CountyParish)
	}
	if s500101.IncidentName != "Hurricane EXERCISE-2026" {
		t.Errorf("merge: 500101 incident = %q", s500101.IncidentName)
	}

	// A spine row with no enrichment match keeps empty enrichment fields.
	orphan := []Shelter{{ShelterID: 999999, Name: "No match"}}
	mergeEnrichment(orphan, em)
	if orphan[0].CountyParish != "" {
		t.Errorf("unmatched spine row got county %q, want empty", orphan[0].CountyParish)
	}

	// A zero-id spine row must never match an em[0] entry (guard).
	zero := []Shelter{{ShelterID: 0, Name: "Zero id"}}
	mergeEnrichment(zero, map[int]Shelter{0: {CountyParish: "SHOULD-NOT-APPLY"}})
	if zero[0].CountyParish != "" {
		t.Errorf("zero-id row matched em[0]: county = %q, want empty", zero[0].CountyParish)
	}
}

// TestParseEnrichmentFailsLoud: a wrong-shape body and an ArcGIS error both error,
// while a real empty result succeeds (never a silent "no enrichment").
func TestParseEnrichmentFailsLoud(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"features":null}`, `{"status":"blocked"}`} {
		if _, err := parseEnrichment([]byte(raw)); err == nil {
			t.Errorf("parseEnrichment(%s) returned no error; want unrecognized-shape error", raw)
		}
	}
	if _, err := parseEnrichment([]byte(`{"error":{"code":400,"message":"bad"}}`)); err == nil {
		t.Error("parseEnrichment(error obj) returned no error")
	}
	em, err := parseEnrichment([]byte(`{"features":[]}`))
	if err != nil || len(em) != 0 {
		t.Errorf("parseEnrichment(empty) = %d,%v; want 0,nil", len(em), err)
	}
}

// TestParseEnrichmentDuplicateIDDropped: a duplicate shelter_id in the feed
// breaks the 1:1 join assumption, so that id must get NO enrichment (honest null)
// rather than a silent last-wins misattribution. A third row for the same id must
// not resurrect it.
func TestParseEnrichmentDuplicateIDDropped(t *testing.T) {
	raw := []byte(`{"features":[
		{"attributes":{"shelter_id":500101,"county_parish":"Harris","incident_name":"A"}},
		{"attributes":{"shelter_id":500102,"county_parish":"Galveston"}},
		{"attributes":{"shelter_id":500101,"county_parish":"Travis","incident_name":"B"}},
		{"attributes":{"shelter_id":500101,"county_parish":"Bexar"}}
	]}`)
	em, err := parseEnrichment(raw)
	if err != nil {
		t.Fatalf("parseEnrichment: %v", err)
	}
	if _, ok := em[500101]; ok {
		t.Errorf("duplicate shelter_id 500101 must be dropped, got %+v", em[500101])
	}
	if s, ok := em[500102]; !ok || s.CountyParish != "Galveston" {
		t.Errorf("non-duplicate 500102 should survive: %+v ok=%v", em[500102], ok)
	}
}

// TestApplyEnrichmentSkipsOffline: --no-enrich and offline data sources must NOT
// fetch (the stub fails if called) and must return a note explaining the absence.
func TestApplyEnrichmentSkipsOffline(t *testing.T) {
	stubEnrichment(t, func(context.Context) (map[int]Shelter, error) {
		t.Fatal("fetchEnrichment must not be called when enrichment is skipped")
		return nil, nil
	})
	cases := []struct {
		name        string
		flags       *rootFlags
		spineSource string
	}{
		{"no-enrich", &rootFlags{noEnrich: true, dataSource: "auto"}, "live"},
		{"data-source-local", &rootFlags{dataSource: "local"}, "live"},
		{"spine-from-local", &rootFlags{dataSource: "auto"}, "local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shelters := []Shelter{{ShelterID: 500101}}
			st := applyEnrichment(context.Background(), tc.flags, shelters, tc.spineSource)
			if st.OK {
				t.Errorf("%s: OK = true, want false", tc.name)
			}
			if st.Note == "" {
				t.Errorf("%s: expected an explanatory note", tc.name)
			}
			if shelters[0].CountyParish != "" {
				t.Errorf("%s: shelter was enriched despite skip", tc.name)
			}
		})
	}
}

// TestApplyEnrichmentSuccessAndFailure exercises the merge-on-success and
// degrade-to-note-on-failure paths.
func TestApplyEnrichmentSuccessAndFailure(t *testing.T) {
	// Success: stub returns one record; it must merge and report OK.
	stubEnrichment(t, func(context.Context) (map[int]Shelter, error) {
		return map[int]Shelter{500101: {ShelterID: 500101, CountyParish: "Harris", GeneratorOnsite: "YES"}}, nil
	})
	shelters := []Shelter{{ShelterID: 500101}, {ShelterID: 500102}}
	st := applyEnrichment(context.Background(), &rootFlags{dataSource: "auto"}, shelters, "live")
	if !st.OK || !st.Attempted {
		t.Fatalf("success: state = %+v, want attempted+ok", st)
	}
	if shelters[0].CountyParish != "Harris" || shelters[0].GeneratorOnsite != "YES" {
		t.Errorf("success: 500101 not enriched: %+v", shelters[0])
	}
	if shelters[1].CountyParish != "" {
		t.Errorf("success: 500102 had no match but got county %q", shelters[1].CountyParish)
	}

	// Failure: stub errors; command must still succeed with a note, no enrichment.
	stubEnrichment(t, func(context.Context) (map[int]Shelter, error) {
		return nil, errors.New("network down")
	})
	shelters = []Shelter{{ShelterID: 500101}}
	st = applyEnrichment(context.Background(), &rootFlags{dataSource: "auto"}, shelters, "live")
	if st.OK {
		t.Error("failure: OK = true, want false")
	}
	if !st.Attempted || st.Note == "" {
		t.Errorf("failure: state = %+v, want attempted with a note", st)
	}
	if shelters[0].CountyParish != "" {
		t.Error("failure: shelter enriched despite fetch error")
	}
}

// TestCountyAndGeneratorFilters confirms the new filters operate on merged
// enrichment fields.
func TestCountyAndGeneratorFilters(t *testing.T) {
	spine := parseFixture(t, syntheticFixture)
	em, err := parseEnrichment(readFixture(t, enrichSyntheticFixture))
	if err != nil {
		t.Fatalf("parseEnrichment: %v", err)
	}
	mergeEnrichment(spine, em)

	// county "harris" (case-insensitive) matches the three Harris-county shelters.
	harris := shelterFilter{county: "harris"}.apply(spine)
	if len(harris) != 3 {
		t.Errorf("county=harris matched %d, want 3", len(harris))
	}
	for _, s := range harris {
		if !strings.EqualFold(s.CountyParish, "Harris") {
			t.Errorf("county filter leaked %q", s.CountyParish)
		}
	}

	// generator: only confirmed-YES shelters.
	gens := shelterFilter{generator: true}.apply(spine)
	if len(gens) == 0 {
		t.Fatal("generator filter matched nothing; synthetic has YES shelters")
	}
	for _, s := range gens {
		if !isYes(s.GeneratorOnsite) {
			t.Errorf("generator filter leaked %q", s.GeneratorOnsite)
		}
	}

	// On a spine with no enrichment, both filters honestly match nothing.
	bare := parseFixture(t, syntheticFixture)
	if got := len(shelterFilter{county: "harris"}.apply(bare)); got != 0 {
		t.Errorf("county filter on un-enriched spine matched %d, want 0", got)
	}
	if got := len(shelterFilter{generator: true}.apply(bare)); got != 0 {
		t.Errorf("generator filter on un-enriched spine matched %d, want 0", got)
	}
}

// TestRenderShelterDetailEnrichment confirms the human detail surfaces the merged
// fields and the enrichment note.
func TestRenderShelterDetailEnrichment(t *testing.T) {
	spine := parseFixture(t, syntheticFixture)
	em, err := parseEnrichment(readFixture(t, enrichSyntheticFixture))
	if err != nil {
		t.Fatalf("parseEnrichment: %v", err)
	}
	mergeEnrichment(spine, em)
	var s Shelter
	for _, sh := range spine {
		if sh.ShelterID == 500101 {
			s = sh
		}
	}
	out := renderShelterDetail(s, "")
	for _, want := range []string{"county     Harris", "incident   Hurricane EXERCISE-2026", "opened     2026-06-11", "generator  YES", "100yr-floodplain=YES", "general=100", "713-555-0101"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q\n---\n%s", want, out)
		}
	}
	// The note appears when enrichment did not fully succeed.
	withNote := renderShelterDetail(s, "Enrichment skipped.")
	if !strings.Contains(withNote, "Enrichment skipped.") {
		t.Error("detail missing enrichment note")
	}
}
