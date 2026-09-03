// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import "testing"

func TestValidatePickupAvailabilityBinding(t *testing.T) {
	address := map[string]any{"streetLines": []any{"1 Test Way"}, "city": "Austin", "stateOrProvinceCode": "TX", "postalCode": "78701", "countryCode": "US"}
	schedule := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             "FDXG",
		"packageCount":            1,
		"totalWeight":             map[string]any{"units": "LB", "value": 2.0},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-04T09:00:00-05:00",
			"customerCloseTime":  "17:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"personName": "Test User", "phoneNumber": "5555550100"},
				"address": address,
			},
		},
	}
	availability := map[string]any{
		"associatedAccountNumber": "123456789",
		"carriers":                []any{"FDXG"},
		"pickupRequestType":       []any{"SAME_DAY"},
		"countryRelationship":     "DOMESTIC",
		"dispatchDate":            "2026-09-04",
		"packageReadyTime":        "09:00:00",
		"customerCloseTime":       "17:00:00",
		"pickupAddress":           map[string]any{"postalCode": "78701", "countryCode": "US"},
	}
	if err := ValidatePickupAvailabilityBinding(schedule, availability); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	availability["pickupAddress"] = address
	if err := ValidatePickupAvailabilityBinding(schedule, availability); err != nil {
		t.Fatalf("matching optional availability address fields rejected: %v", err)
	}
	availability["carriers"] = []any{"FDXG", "FDXE"}
	if err := ValidatePickupAvailabilityBinding(schedule, availability); err == nil {
		t.Fatal("multi-carrier availability request accepted")
	}
	availability["carriers"] = []any{"FDXG"}
	availability["dispatchDate"] = "2026-09-05"
	if err := ValidatePickupAvailabilityBinding(schedule, availability); err == nil {
		t.Fatal("mismatched dispatch date accepted")
	}
	availability["dispatchDate"] = "2026-09-04"
	availability["pickupAddress"] = map[string]any{"postalCode": "78701", "countryCode": "US", "city": "Dallas"}
	if err := ValidatePickupAvailabilityBinding(schedule, availability); err == nil {
		t.Fatal("mismatched optional pickup address field accepted")
	}
}

func TestMatchingPickupAvailabilityRequiresOneCorrelatedOption(t *testing.T) {
	response := []byte(`{"output":{"options":[{"carrier":"FDXG","available":true,"pickupDate":"2026-09-04","scheduleDay":"FRI","cutOffTime":"16:00"},{"carrier":"FDXG","available":false,"pickupDate":"2026-09-04","scheduleDay":"FRI","cutOffTime":"18:00"}]}}`)
	if _, err := matchingPickupAvailability(response, "FDXG", "2026-09-04"); err == nil {
		t.Fatal("multiple matching pickup availability options accepted")
	}
	response = []byte(`{"output":{"options":[{"carrier":"FDXE","available":true,"pickupDate":"2026-09-04","scheduleDay":"FRI"}]}}`)
	if _, err := matchingPickupAvailability(response, "FDXG", "2026-09-04"); err == nil {
		t.Fatal("unmatched pickup availability option accepted")
	}
}
