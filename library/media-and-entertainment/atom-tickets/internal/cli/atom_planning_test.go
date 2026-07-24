// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestParseAtomCoordinatesAcceptsNegativeLongitude(t *testing.T) {
	lat, lon, err := parseAtomCoordinates("40.7505", "-73.9934")
	if err != nil {
		t.Fatalf("parseAtomCoordinates() error = %v", err)
	}
	if lat != 40.7505 || lon != -73.9934 {
		t.Fatalf("coordinates = %v,%v", lat, lon)
	}
}

func TestParseAtomCoordinatesRejectsOutOfRange(t *testing.T) {
	if _, _, err := parseAtomCoordinates("91", "0"); err == nil {
		t.Fatal("expected out-of-range latitude error")
	}
	if _, _, err := parseAtomCoordinates("0", "-181"); err == nil {
		t.Fatal("expected out-of-range longitude error")
	}
}

func TestFindObjectSliceHandlesDocumentedVenueEnvelope(t *testing.T) {
	raw := json.RawMessage(`{"venues":[{"id":"C1","name":"Atom Cinema","kmDistance":2.5}]}`)
	venues := findObjectSlice(raw, "venues", "venueDetails")
	if len(venues) != 1 || anyString(venues[0], "id") != "C1" {
		t.Fatalf("venues = %#v", venues)
	}
}

func TestLowestOfferUsesLowestPositiveAdvertisedPrice(t *testing.T) {
	item := map[string]any{
		"offers": []any{
			map[string]any{"price": map[string]any{"amount": 18.0, "currency": "USD"}},
			map[string]any{"price": map[string]any{"amount": 11.5, "currency": "USD"}},
			map[string]any{"price": map[string]any{"amount": 0.0, "currency": "USD"}},
		},
	}
	price, currency := lowestOffer(item)
	if price != 11.5 || currency != "USD" {
		t.Fatalf("lowestOffer() = %v %q", price, currency)
	}
}

func TestCollectNamedObjectsFindsNestedShowtimesOnce(t *testing.T) {
	root := map[string]any{
		"venues": map[string]any{
			"C1": map[string]any{
				"showtimes": []any{map[string]any{"id": "S1", "isoStartTime": "2026-07-24T20:00:00Z"}},
			},
		},
	}
	var ids []string
	collectNamedObjects(root, []string{"showtimes"}, func(item map[string]any) {
		ids = append(ids, anyString(item, "id"))
	})
	if len(ids) != 1 || ids[0] != "S1" {
		t.Fatalf("ids = %#v", ids)
	}
}
