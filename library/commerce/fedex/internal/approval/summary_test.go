// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package approval

import (
	"strings"
	"testing"
)

func TestCreateLabelSummaryIncludesRedactedDestinationAndWeight(t *testing.T) {
	request := map[string]any{
		"accountNumber": map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"serviceType":   "FEDEX_GROUND",
			"shipDatestamp": "2026-09-03",
			"recipients": []any{map[string]any{
				"contact": map[string]any{"personName": "sentinel-recipient"},
				"address": map[string]any{
					"streetLines":         []any{"sentinel-street"},
					"city":                "Austin",
					"stateOrProvinceCode": "TX",
					"postalCode":          "78701",
					"countryCode":         "US",
				},
			}},
			"requestedPackageLineItems": []any{
				map[string]any{"weight": map[string]any{"value": 10.0, "units": "LB"}},
				map[string]any{"weight": map[string]any{"value": 15.0, "units": "LB"}},
			},
		},
	}

	summary := Summarize("create_label", request)
	if summary.DestinationRegion != "Austin, TX, US, ***701" {
		t.Fatalf("unexpected destination summary: %q", summary.DestinationRegion)
	}
	if summary.WeightSummary != "25 LB" || summary.PackageCount != 2 || summary.ServiceType != "FEDEX_GROUND" {
		t.Fatalf("incomplete label summary: %#v", summary)
	}
	serialized := strings.Join([]string{summary.DestinationRegion, summary.WeightSummary, summary.ServiceType}, " ")
	if strings.Contains(serialized, "sentinel-recipient") || strings.Contains(serialized, "sentinel-street") {
		t.Fatalf("summary leaked recipient PII: %s", serialized)
	}
}

func TestPickupSummaryIncludesRedactedOriginWindowAndWeight(t *testing.T) {
	request := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             "FDXE",
		"packageCount":            3.0,
		"totalWeight":             map[string]any{"value": 42.0, "units": "LB"},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-03T10:00:00-05:00",
			"customerCloseTime":  "17:00:00",
			"pickupAddress": map[string]any{
				"streetLines":         []any{"sentinel-pickup-street"},
				"city":                "Plano",
				"stateOrProvinceCode": "TX",
				"postalCode":          "75024",
				"countryCode":         "US",
			},
		},
	}

	summary := Summarize("schedule_pickup", request)
	if summary.DestinationRegion != "Plano, TX, US, ***024" || summary.PackageCount != 3 || summary.WeightSummary != "42 LB" {
		t.Fatalf("incomplete pickup summary: %#v", summary)
	}
	if !strings.Contains(summary.PickupWindow, "2026-09-03T10:00:00-05:00") || !strings.Contains(summary.PickupWindow, "17:00:00") {
		t.Fatalf("pickup window missing: %#v", summary)
	}
	if strings.Contains(summary.DestinationRegion, "sentinel-pickup-street") {
		t.Fatalf("pickup summary leaked street: %#v", summary)
	}
}
