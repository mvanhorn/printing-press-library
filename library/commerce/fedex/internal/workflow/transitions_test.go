// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

func TestBeginPickupCancellationUsesCompareAndSet(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ctx := context.Background()
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertPickup(ctx, store.Pickup{OperationID: "create-op", ConfirmationNumber: "PU123", AccountNumber: "123456789", CarrierCode: "FDXG", ScheduledDate: "2026-09-04", RequestHash: "create-hash", Status: "scheduled"}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()
	body := map[string]any{"associatedAccountNumber": map[string]any{"value": "123456789"}, "pickupConfirmationCode": "PU123", "carrierCode": "FDXG", "scheduledDate": "2026-09-04"}
	if result, err := BeginMutation(ctx, ActionCancelPickup, body, PersistOptions{OperationID: "cancel-op"}); err != nil || result != nil {
		t.Fatalf("first begin result=%v err=%v", result, err)
	}
	if _, err := BeginMutation(ctx, ActionCancelPickup, body, PersistOptions{OperationID: "cancel-op-2"}); err == nil {
		t.Fatal("second cancellation begin should require reconciliation")
	}
	PersistRejected(ctx, ActionCancelPickup, body, PersistOptions{})
	if result, err := BeginMutation(ctx, ActionCancelPickup, body, PersistOptions{OperationID: "cancel-op-3"}); err != nil || result != nil {
		t.Fatalf("retry after definite rejection result=%v err=%v", result, err)
	}
}

func TestBeginAndCompletePickupCreationPreservesAttempt(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ctx := context.Background()
	typedBody := validSchedulePickupRequest()
	encodedBody, err := json.Marshal(typedBody)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(encodedBody, &body); err != nil {
		t.Fatal(err)
	}
	options := PersistOptions{OperationID: "pickup-op", PickupPreflight: "verified", PickupPreflightCutoff: "17:00", PickupPreflightAccessStart: "09:00"}
	if result, err := BeginMutation(ctx, ActionSchedulePickup, body, options); err != nil || result != nil {
		t.Fatalf("begin result=%v err=%v", result, err)
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	pickup, err := ledger.GetPickupByOperationID(ctx, "pickup-op")
	_ = ledger.Close()
	if err != nil || pickup == nil || pickup.Status != "executing" {
		t.Fatalf("executing pickup=%+v err=%v", pickup, err)
	}
	response := json.RawMessage(`{"customerTransactionId":"tx-pickup","output":{"pickupConfirmationCode":"PU123","scheduledDate":"2026-09-03","location":"ABCD"}}`)
	if _, err := PersistSuccess(ctx, ActionSchedulePickup, body, response, options); err != nil {
		t.Fatal(err)
	}
	ledger, _ = store.Open(store.DefaultPath())
	pickup, err = ledger.GetPickupByOperationID(ctx, "pickup-op")
	_ = ledger.Close()
	if err != nil || pickup == nil || pickup.Status != "scheduled" || pickup.ConfirmationNumber != "PU123" {
		t.Fatalf("scheduled pickup=%+v err=%v", pickup, err)
	}
}

func TestUnusablePickupSuccessReportsOperationID(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ctx := context.Background()
	body := schedulePickupBody(t)
	options := PersistOptions{OperationID: "pickup-ambiguous", PickupPreflight: "verified"}
	if _, err := BeginMutation(ctx, ActionSchedulePickup, body, options); err != nil {
		t.Fatal(err)
	}
	_, err := Finish(ctx, ActionSchedulePickup, body, []byte(`{"output":{"message":"no confirmation"}}`), nil, false, options)
	var pickup *PickupOperationError
	if !errors.As(err, &pickup) || pickup.OperationID != options.OperationID || !strings.Contains(err.Error(), options.OperationID) {
		t.Fatalf("unusable success err=%v", err)
	}
	unknown := &client.OutcomeUnknownError{Method: "POST", Path: "/pickup/v1/pickups", StatusCode: 500, Cause: errors.New("unavailable")}
	_, err = Finish(ctx, ActionSchedulePickup, body, nil, unknown, true, PersistOptions{OperationID: "pickup-transport", PickupPreflight: "verified"})
	pickup = nil
	if !errors.As(err, &pickup) || pickup.OperationID != "pickup-transport" || !strings.Contains(err.Error(), "pickup-transport") {
		t.Fatalf("transport failure err=%v", err)
	}
}

func TestCancelPickupReconcileRequiresConfirmation(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ctx := context.Background()
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"unknown-a", "unknown-b"} {
		if err := ledger.UpsertPickup(ctx, store.Pickup{OperationID: id, AccountNumber: "123456789", CarrierCode: "FDXG", ScheduledDate: "2026-09-04", RequestHash: "hash-" + id, Status: "outcome_unknown", PreflightStatus: "verified"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileOperationalState(ctx, ActionCancelPickup, "", "", "", "succeeded"); err == nil {
		t.Fatal("blank pickup confirmation reconciled")
	}
	ledger, err = store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	for _, id := range []string{"unknown-a", "unknown-b"} {
		pickup, err := ledger.GetPickupByOperationID(ctx, id)
		if err != nil || pickup == nil || pickup.Status != "outcome_unknown" {
			t.Fatalf("pickup %s after blank reconcile: %+v err=%v", id, pickup, err)
		}
	}
}

func schedulePickupBody(t *testing.T) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(validSchedulePickupRequest())
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestNotExecutedReconciliationReleasesPickupLedgerForRetry(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ctx := context.Background()
	typedBody := validSchedulePickupRequest()
	encodedBody, err := json.Marshal(typedBody)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(encodedBody, &body); err != nil {
		t.Fatal(err)
	}
	options := PersistOptions{OperationID: "pickup-unknown", PickupPreflight: "verified"}
	if _, err := BeginMutation(ctx, ActionSchedulePickup, body, options); err != nil {
		t.Fatal(err)
	}
	PersistOutcomeUnknown(ctx, ActionSchedulePickup, body, options)
	if err := ReconcileOperationalState(ctx, ActionSchedulePickup, options.OperationID, "", "", "not_executed"); err != nil {
		t.Fatal(err)
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	pickup, err := ledger.GetPickupByOperationID(ctx, options.OperationID)
	_ = ledger.Close()
	if err != nil || pickup == nil || pickup.Status != "rejected" {
		t.Fatalf("reconciled pickup=%+v err=%v", pickup, err)
	}
	if _, err := BeginMutation(ctx, ActionSchedulePickup, body, PersistOptions{OperationID: "pickup-retry", PickupPreflight: "verified"}); err != nil {
		t.Fatalf("retry after not_executed reconciliation: %v", err)
	}
}
