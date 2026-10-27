// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPersistExpressPickupRequiresCancellationLocation(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	typed := validSchedulePickupRequest()
	typed.CarrierCode = "FDXE"
	data, err := json.Marshal(typed)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	response := json.RawMessage(`{"transactionId":"express-tx","output":{"pickupConfirmationCode":"EX123","scheduledDate":"2026-09-03"}}`)
	_, err = PersistSuccess(context.Background(), ActionSchedulePickup, body, response, PersistOptions{OperationID: "express-op", PickupPreflight: "verified"})
	if err == nil || !strings.Contains(err.Error(), "location code") {
		t.Fatalf("missing Express location error=%v", err)
	}
}
