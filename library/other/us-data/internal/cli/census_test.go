// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestResolveKnownPlace(t *testing.T) {
	ref, ok := resolvePlace("Austin, TX")
	if !ok {
		t.Fatal("Austin, TX was not resolved")
	}
	if ref.State != "48" || ref.Place != "05000" {
		t.Fatalf("resolved Austin as state=%s place=%s", ref.State, ref.Place)
	}
}

func TestParsePopulation(t *testing.T) {
	ref := placeRef{Name: "Austin city, Texas", State: "48", Place: "05000"}
	body := []byte(`[["NAME","DP05_0001E","state","place"],["Austin city, Texas","984567","48","05000"]]`)

	result, err := parsePopulation(ref, body)
	if err != nil {
		t.Fatalf("parsePopulation returned error: %v", err)
	}
	if result.Population != "984567" {
		t.Fatalf("population = %s, want 984567", result.Population)
	}
	if result.Place != "Austin city, Texas" {
		t.Fatalf("place = %s", result.Place)
	}
}
