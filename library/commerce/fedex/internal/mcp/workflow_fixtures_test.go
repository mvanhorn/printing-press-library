// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

func validMCPCreateLabelRequest() map[string]any {
	return map[string]any{
		"labelResponseOptions": "LABEL",
		"accountNumber":        map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"shipper": map[string]any{
				"contact": map[string]any{"personName": "Sender", "phoneNumber": "5550000000"},
				"address": map[string]any{"streetLines": []any{"1 Origin St"}, "city": "Origin", "postalCode": "00000", "countryCode": "US"},
			},
			"recipients": []any{map[string]any{
				"contact": map[string]any{"personName": "Recipient", "phoneNumber": "5550000001"},
				"address": map[string]any{"streetLines": []any{"2 Destination St"}, "city": "Destination", "postalCode": "00001", "countryCode": "US"},
			}},
			"serviceType":               "FEDEX_GROUND",
			"packagingType":             "YOUR_PACKAGING",
			"requestedPackageLineItems": []any{map[string]any{"weight": map[string]any{"units": "LB", "value": 1}}},
			"labelSpecification":        map[string]any{"imageType": "PDF", "labelStockType": "PAPER_4X6"},
		},
	}
}

func validMCPSchedulePickupRequest() map[string]any {
	return map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             "FDXG",
		"packageCount":            1,
		"totalWeight":             map[string]any{"units": "LB", "value": 1},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-03T09:00:00-05:00",
			"customerCloseTime":  "17:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"personName": "Warehouse", "phoneNumber": "5550000000"},
				"address": map[string]any{"streetLines": []any{"1 Origin St"}, "city": "Origin", "postalCode": "00000", "countryCode": "US"},
			},
		},
	}
}
