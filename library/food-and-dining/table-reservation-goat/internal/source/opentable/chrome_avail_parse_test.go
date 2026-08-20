// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package opentable

import "testing"

func TestParseAvailabilityResponseCurrentEnvelope(t *testing.T) {
	body := []byte(`{
		"data": {
			"availability": [{
				"restaurantId": 486349,
				"availabilityDays": [{
					"dayOffset": 0,
					"slots": [{
						"isAvailable": true,
						"timeOffsetMinutes": 0,
						"slotHash": "slot-hash",
						"slotAvailabilityToken": "slot-token"
					}]
				}]
			}]
		}
	}`)

	got, err := parseAvailabilityResponse(body)
	if err != nil {
		t.Fatalf("parseAvailabilityResponse() error = %v", err)
	}
	if len(got) != 1 || got[0].RestaurantID != 486349 {
		t.Fatalf("parseAvailabilityResponse() = %#v, want restaurant 486349", got)
	}
	if len(got[0].AvailabilityDays) != 1 || len(got[0].AvailabilityDays[0].Slots) != 1 {
		t.Fatalf("parseAvailabilityResponse() = %#v, want one day with one slot", got)
	}
	slot := got[0].AvailabilityDays[0].Slots[0]
	if !slot.IsAvailable || slot.SlotHash != "slot-hash" || slot.SlotAvailabilityToken != "slot-token" {
		t.Fatalf("slot = %#v, want current-envelope slot fields preserved", slot)
	}
}
