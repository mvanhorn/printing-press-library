package instacart

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNewShopCollectionScopedVars_OmitsCoordinatesWhenUnset pins the fix for
// mvanhorn/printing-press-library#501: when the user has only configured
// postal_code (no latitude/longitude), the marshalled JSON must NOT contain
// a "coordinates" key, instead of sending {"coordinates":{"latitude":0,"longitude":0}}
// which Instacart's UsersCoordinatesInput rejects as invalid.
func TestNewShopCollectionScopedVars_OmitsCoordinatesWhenUnset(t *testing.T) {
	v := NewShopCollectionScopedVars("qfc", "98052", "", 0, 0)
	if v.Coordinates != nil {
		t.Fatalf("Coordinates should be nil when both lat/lng are 0, got %+v", v.Coordinates)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "coordinates") {
		t.Fatalf("JSON should not contain a coordinates key when both lat/lng are 0, got: %s", string(out))
	}
}

func TestNewShopCollectionScopedVars_IncludesCoordinatesWhenSet(t *testing.T) {
	tests := []struct {
		name     string
		lat, lng float64
	}{
		{"both set", 47.6740, -122.1215},
		{"latitude only", 47.6740, 0},
		{"longitude only", 0, -122.1215},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewShopCollectionScopedVars("qfc", "98052", "", tc.lat, tc.lng)
			if v.Coordinates == nil {
				t.Fatalf("Coordinates should be non-nil when at least one of lat/lng is non-zero")
			}
			if v.Coordinates.Latitude != tc.lat || v.Coordinates.Longitude != tc.lng {
				t.Fatalf("Coordinates round-trip: want lat=%v lng=%v, got %+v", tc.lat, tc.lng, v.Coordinates)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(out), `"coordinates"`) {
				t.Fatalf("JSON should contain a coordinates key when set, got: %s", string(out))
			}
		})
	}
}

func TestNewShopCollectionScopedVars_OmitsEmptyAddressID(t *testing.T) {
	v := NewShopCollectionScopedVars("qfc", "98052", "", 47.6740, -122.1215)
	if v.AddressID != "" {
		t.Fatalf("AddressID should be empty string, got %q", v.AddressID)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// addressId has omitempty on the struct tag; empty string should drop.
	if strings.Contains(string(out), `"addressId"`) {
		t.Fatalf("JSON should omit addressId when empty, got: %s", string(out))
	}
}

func TestNewShopCollectionScopedVars_RequiredFieldsRoundTrip(t *testing.T) {
	v := NewShopCollectionScopedVars("qfc", "98052", "addr-123", 47.6740, -122.1215)
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	for _, want := range []string{`"retailerSlug":"qfc"`, `"postalCode":"98052"`, `"addressId":"addr-123"`, `"coordinates":`, `"allowCanonicalFallback":false`} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON missing %q; full: %s", want, got)
		}
	}
}
