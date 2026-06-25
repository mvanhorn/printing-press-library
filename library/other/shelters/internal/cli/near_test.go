// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
)

// stubGeocoder swaps geocodeOneLine for a deterministic table for the test and
// restores it afterward. Addresses not in the table resolve as "no match"
// (ok=false), exercising the skip-with-count path.
func stubGeocoder(t *testing.T, table map[string]latlon) {
	t.Helper()
	prev := geocodeOneLine
	t.Cleanup(func() { geocodeOneLine = prev })
	geocodeOneLine = func(_ context.Context, oneLine string) (latlon, bool, error) {
		if ll, ok := table[strings.TrimSpace(oneLine)]; ok {
			return ll, true, nil
		}
		return latlon{}, false, nil
	}
}

func TestResolveOrigin(t *testing.T) {
	stubGeocoder(t, map[string]latlon{"downtown": {Lat: 30, Lon: -95}})

	o, err := resolveOrigin(context.Background(), "29.76,-95.37")
	if err != nil {
		t.Fatalf("lat,lon origin: %v", err)
	}
	if o.Geocoded || o.Latitude != 29.76 || o.Longitude != -95.37 {
		t.Fatalf("lat,lon origin parsed wrong: %+v", o)
	}

	if _, err := resolveOrigin(context.Background(), "200,0"); err == nil {
		t.Fatal("expected out-of-range error for lat 200")
	}

	o, err = resolveOrigin(context.Background(), "downtown")
	if err != nil || !o.Geocoded || o.Latitude != 30 {
		t.Fatalf("geocoded origin wrong: %+v err=%v", o, err)
	}

	if _, err := resolveOrigin(context.Background(), "nowhere-real"); err == nil {
		t.Fatal("expected error when origin cannot be geocoded")
	}
}

// keyFor builds the geocoder table key for a shelter by id from the parsed feed.
func keyFor(t *testing.T, shelters []Shelter, id int) string {
	t.Helper()
	for _, s := range shelters {
		if s.ShelterID == id {
			return shelterOneLine(s)
		}
	}
	t.Fatalf("shelter id %d not in fixture", id)
	return ""
}

func TestBuildNearRanksAndGeocodes(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	// Resolve the 3 geocodable null-coord shelters; leave id 500112 (Cameron,
	// "Unmarked county road") out so it lands in unlocated.
	stubGeocoder(t, map[string]latlon{
		keyFor(t, shelters, 500109): {Lat: 29.690, Lon: -95.199}, // Pasadena, ~11 mi
		keyFor(t, shelters, 500110): {Lat: 30.090, Lon: -93.730}, // Orange, far
		keyFor(t, shelters, 500111): {Lat: 30.270, Lon: -89.780}, // Slidell, far
	})

	origin := originInfo{Query: "houston", Latitude: 29.76, Longitude: -95.37}
	d := buildNear(context.Background(), origin, shelters, 0, 0)

	if d.UnlocatedCount != 1 {
		t.Errorf("unlocated_count = %d, want 1 (Cameron)", d.UnlocatedCount)
	}
	if d.LocatedCount != 11 {
		t.Errorf("located_count = %d, want 11 (8 direct + 3 geocoded)", d.LocatedCount)
	}
	// Sorted ascending: nearest must be Bayou Civic Center (id 500101, ~0 mi).
	if len(d.Shelters) == 0 || d.Shelters[0].ShelterID != 500101 {
		t.Fatalf("nearest shelter = %+v, want id 500101", d.Shelters[0])
	}
	if d.Shelters[0].DistanceMiles > 0.5 {
		t.Errorf("nearest distance = %.2f, want ~0", d.Shelters[0].DistanceMiles)
	}
	for i := 1; i < len(d.Shelters); i++ {
		if d.Shelters[i].DistanceMiles < d.Shelters[i-1].DistanceMiles {
			t.Fatalf("results not sorted ascending at %d: %.2f < %.2f", i, d.Shelters[i].DistanceMiles, d.Shelters[i-1].DistanceMiles)
		}
	}
	// The geocoded shelters must be flagged.
	var sawGeocoded bool
	for _, s := range d.Shelters {
		if s.ShelterID == 500109 {
			sawGeocoded = s.CoordsGeocoded
		}
	}
	if !sawGeocoded {
		t.Error("expected shelter 500109 to be flagged coords_geocoded")
	}
}

func TestBuildNearMaxMilesAndLimit(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	stubGeocoder(t, map[string]latlon{}) // resolve nothing; only direct-coord shelters rank

	origin := originInfo{Query: "houston", Latitude: 29.76, Longitude: -95.37}

	// max-miles 6: only Bayou (~0) and Westside (~5.6) qualify.
	d := buildNear(context.Background(), origin, shelters, 6, 0)
	if d.LocatedCount != 2 {
		t.Errorf("max-miles 6 located_count = %d, want 2", d.LocatedCount)
	}
	for _, s := range d.Shelters {
		if s.DistanceMiles > 6 {
			t.Errorf("shelter %d at %.2f mi exceeds max-miles 6", s.ShelterID, s.DistanceMiles)
		}
	}

	// limit 1: single nearest result.
	d = buildNear(context.Background(), origin, shelters, 0, 1)
	if len(d.Shelters) != 1 {
		t.Errorf("limit 1 returned %d shelters", len(d.Shelters))
	}
}

// TestNearNoCoordCollisionOnZeroID guards the safety fix: two distinct shelters
// that both default to ShelterID==0 (feed omitted shelter_id) must each keep
// their OWN coordinates, never clobber each other through an id-keyed map.
func TestNearNoCoordCollisionOnZeroID(t *testing.T) {
	seattleLat, seattleLon := 47.6062, -122.3321
	alpha := Shelter{ShelterID: 0, Name: "Alpha (Seattle)", Latitude: &seattleLat, Longitude: &seattleLon}
	beta := Shelter{ShelterID: 0, Name: "Beta (Houston addr)", Address: "100 Main", City: "Houston", State: "TX", Zip: "77002"}
	stubGeocoder(t, map[string]latlon{shelterOneLine(beta): {Lat: 29.760, Lon: -95.370}})

	origin := originInfo{Query: "houston", Latitude: 29.76, Longitude: -95.37}
	d := buildNear(context.Background(), origin, []Shelter{alpha, beta}, 0, 0)

	if d.LocatedCount != 2 {
		t.Fatalf("located_count = %d, want 2", d.LocatedCount)
	}
	byName := map[string]shelterDistance{}
	for _, s := range d.Shelters {
		byName[s.Name] = s
	}
	a, b := byName["Alpha (Seattle)"], byName["Beta (Houston addr)"]
	if a.Latitude == nil || *a.Latitude != seattleLat {
		t.Errorf("Alpha lost its own coordinates: %+v", a.Latitude)
	}
	if a.CoordsGeocoded {
		t.Error("Alpha (real coords) must not be flagged geocoded")
	}
	if a.DistanceMiles < 1000 {
		t.Errorf("Alpha distance = %.1f mi; Seattle->Houston should be far, not borrowed from Beta", a.DistanceMiles)
	}
	if b.DistanceMiles > 1 {
		t.Errorf("Beta distance = %.1f mi, want ~0 (its geocoded Houston coords)", b.DistanceMiles)
	}
}

func TestNearPetFilterComposesWithDistance(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	stubGeocoder(t, map[string]latlon{})
	pets := shelterFilter{pets: true}.apply(shelters)
	origin := originInfo{Query: "houston", Latitude: 29.76, Longitude: -95.37}
	d := buildNear(context.Background(), origin, pets, 0, 0)
	for _, s := range d.Shelters {
		if !allowsPets(s.PetAccommodations) {
			t.Errorf("near --pets returned non-pet shelter %d (%s)", s.ShelterID, s.PetAccommodations)
		}
	}
}
