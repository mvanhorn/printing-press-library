// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestFindShelterRejectsNonPositiveID locks the union regression: Red Cross-only
// shelters are appended with shelter_id 0 (the RC feed has no FEMA id), so `shelter 0`
// (or any non-positive id) must report not-found instead of returning the first such row.
func TestFindShelterRejectsNonPositiveID(t *testing.T) {
	shelters := []Shelter{
		{ShelterID: 0, Name: "RC Only", Source: "redcross"},
		{ShelterID: 368133, Name: "Lincoln Community Center", Source: "fema"},
	}
	if _, ok := findShelter(shelters, 0); ok {
		t.Error("shelter 0 must not match the Red Cross-only (id 0) row")
	}
	if _, ok := findShelter(shelters, -5); ok {
		t.Error("a negative id must not match")
	}
	if s, ok := findShelter(shelters, 368133); !ok || s.Name != "Lincoln Community Center" {
		t.Errorf("positive id 368133 should match the FEMA row, got ok=%v name=%q", ok, s.Name)
	}
}
