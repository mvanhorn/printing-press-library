// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

const (
	syntheticFixture = "testdata/openshelters-active-SYNTHETIC.json"
	liveFixture      = "testdata/openshelters-live-2026-06-16.json"
)

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	return b
}

func parseFixture(t *testing.T, path string) []Shelter {
	t.Helper()
	s, err := parseShelters(readFixture(t, path))
	if err != nil {
		t.Fatalf("parseShelters(%s): %v", path, err)
	}
	return s
}

// TestHaversineCanonicalVector checks the distance math against an INDEPENDENT
// published ground-truth vector (Wikipedia haversine example), not against
// numbers derived from our own fixtures. This catches radians/degrees, wrong
// radius, and swapped lat/lon bugs.
func TestHaversineCanonicalVector(t *testing.T) {
	got := haversineMiles(36.12, -86.67, 33.94, -118.40)
	const want = 1793.56 // 2887.2599 km at R=3958.7613 mi
	if math.Abs(got-want) > 0.5 {
		t.Fatalf("haversineMiles canonical vector = %.2f mi, want ~%.2f mi", got, want)
	}
	// Symmetry: distance is the same in both directions.
	rev := haversineMiles(33.94, -118.40, 36.12, -86.67)
	if math.Abs(got-rev) > 1e-6 {
		t.Fatalf("haversine not symmetric: %.6f vs %.6f", got, rev)
	}
	// Identity: zero distance to self.
	if d := haversineMiles(40, -90, 40, -90); d != 0 {
		t.Fatalf("haversine to self = %v, want 0", d)
	}
}

func TestAllowsPets(t *testing.T) {
	cases := map[string]bool{
		"COHABIT": true, "ONSITE": true,
		"cohabit": true, " onsite ": true, // normalization
		"NONE": false, "None": false, "": false, " ": false, "NO": false, "UNK": false,
	}
	for code, want := range cases {
		if got := allowsPets(code); got != want {
			t.Errorf("allowsPets(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestIsYes(t *testing.T) {
	cases := map[string]bool{
		"YES": true, "yes": true, " Yes ": true,
		"NO": false, "UNK": false, "": false, " ": false,
	}
	for code, want := range cases {
		if got := isYes(code); got != want {
			t.Errorf("isYes(%q) = %v, want %v", code, got, want)
		}
	}
}

// TestParseLiveFixtureSparse pins the REAL quiet-state contract: 6 open
// shelters, every coded field NONE/UNK, every numeric/geo field null.
func TestParseLiveFixtureSparse(t *testing.T) {
	shelters := parseFixture(t, liveFixture)
	if len(shelters) != 6 {
		t.Fatalf("live fixture: got %d shelters, want 6", len(shelters))
	}
	for _, s := range shelters {
		if s.Status != "OPEN" {
			t.Errorf("%d: status = %q, want OPEN", s.ShelterID, s.Status)
		}
		if s.PetAccommodations != "NONE" {
			t.Errorf("%d: pets = %q, want NONE", s.ShelterID, s.PetAccommodations)
		}
		if s.Latitude != nil || s.Longitude != nil {
			t.Errorf("%d: expected null coords, got %v,%v", s.ShelterID, s.Latitude, s.Longitude)
		}
		if s.TotalPopulation != nil || s.EvacuationCapacity != nil {
			t.Errorf("%d: expected null population/capacity", s.ShelterID)
		}
		if s.Address == "" {
			t.Errorf("%d: expected a street address for geocoding", s.ShelterID)
		}
	}
}

// TestParseSyntheticNormalizes confirms codes are normalized and shelter_id is
// the stable join key.
func TestParseSyntheticNormalizes(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	if len(shelters) != 12 {
		t.Fatalf("synthetic fixture: got %d shelters, want 12", len(shelters))
	}
	pet, full := 0, 0
	for _, s := range shelters {
		if allowsPets(s.PetAccommodations) {
			pet++
		}
		if s.Status == "FULL" {
			full++
		}
		if s.ShelterID == 0 {
			t.Errorf("shelter with zero shelter_id (missing stable key): %+v", s.Name)
		}
	}
	if pet != 7 {
		t.Errorf("pet-friendly count = %d, want 7", pet)
	}
	if full != 2 {
		t.Errorf("FULL count = %d, want 2", full)
	}
}

func TestParseArcgisError(t *testing.T) {
	_, err := parseShelters([]byte(`{"error":{"code":400,"message":"Invalid query"}}`))
	if err == nil {
		t.Fatal("expected an error for an ArcGIS error response, got nil")
	}
}

// TestParseRejectsUnrecognizedShape: valid JSON with no features and no error
// must fail loudly, not be reported as "0 open shelters" (a broken feed must
// never read as a quiet day). A real empty result must still succeed.
func TestParseRejectsUnrecognizedShape(t *testing.T) {
	unrecognized := []string{`{}`, `null`, `{"features":null}`, `{"status":"blocked"}`}
	for _, raw := range unrecognized {
		if _, err := parseShelters([]byte(raw)); err == nil {
			t.Errorf("parseShelters(%s) returned no error; want unrecognized-shape error", raw)
		}
	}
	emptyOK := []string{`{"features":[]}`, `[]`}
	for _, raw := range emptyOK {
		s, err := parseShelters([]byte(raw))
		if err != nil {
			t.Errorf("parseShelters(%s) errored on a real empty result: %v", raw, err)
		}
		if len(s) != 0 {
			t.Errorf("parseShelters(%s) = %d shelters, want 0", raw, len(s))
		}
	}
}

func TestParseBareFeaturesArray(t *testing.T) {
	// The generated response_path can strip to a bare features array.
	raw := []byte(`[{"attributes":{"shelter_id":1,"shelter_name":"X","pet_accommodations_code":"cohabit"}}]`)
	shelters, err := parseShelters(raw)
	if err != nil {
		t.Fatalf("parseShelters(bare array): %v", err)
	}
	if len(shelters) != 1 || shelters[0].PetAccommodations != "COHABIT" {
		t.Fatalf("bare array parse wrong: %+v", shelters)
	}
}
