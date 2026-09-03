// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"
)

func TestStoreNeverPersistsRawFedExResponses(t *testing.T) {
	state, err := Open(t.TempDir() + "/fedex.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer state.Close()

	ctx := context.Background()
	if _, err := state.InsertShipment(ctx, Shipment{TrackingNumber: "synthetic-tracking", RawResponse: "SENTINEL RAW SHIPMENT"}); err != nil {
		t.Fatalf("InsertShipment: %v", err)
	}
	if err := state.InsertRateQuote(ctx, RateQuote{RawResponse: "SENTINEL RAW RATE"}); err != nil {
		t.Fatalf("InsertRateQuote: %v", err)
	}
	if err := state.InsertAddressValidation(ctx, AddressValidationCache{CacheKey: "k", RawResponse: "SENTINEL RAW ADDRESS"}); err != nil {
		t.Fatalf("InsertAddressValidation: %v", err)
	}

	for _, table := range []string{"shipments", "rate_quotes", "address_validations"} {
		var count int
		if err := state.db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE raw_response <> ''").Scan(&count); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s persisted %d raw response(s)", table, count)
		}
	}
}
