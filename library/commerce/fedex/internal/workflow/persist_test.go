// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

func TestPersistLabelAndPickupOperationalState(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "fedex")
	t.Setenv("FEDEX_DATA_DIR", dataDir)
	ctx := context.Background()

	labelBody := requestMap(t, validCreateLabelRequest())
	labelResponse := []byte(`{"transactionId":"ship-tx","output":{"transactionShipments":[{"masterTrackingNumber":"123456789012","serviceType":"FEDEX_GROUND","pieceResponses":[{"trackingNumber":"123456789012","packageDocuments":[{"contentType":"application/pdf","docType":"LABEL","encodedLabel":"JVBERi0xLjQKJSVFT0YK"}]}]}]}}`)
	result, err := PersistSuccess(ctx, ActionCreateLabel, labelBody, labelResponse, PersistOptions{OperationID: "ship-op"})
	if err != nil {
		t.Fatalf("PersistSuccess(label): %v", err)
	}
	receipt := result.(LabelReceipt)
	if receipt.TrackingNumber != "123456789012" || receipt.TransactionID != "ship-tx" {
		t.Fatalf("label receipt=%#v", receipt)
	}
	labelData, err := os.ReadFile(receipt.LabelPath)
	if err != nil {
		t.Fatalf("read label: %v", err)
	}
	if string(labelData) != "%PDF-1.4\n%%EOF\n" {
		t.Fatalf("label bytes=%q", labelData)
	}
	info, err := os.Stat(receipt.LabelPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("label mode=%v err=%v", info.Mode().Perm(), err)
	}

	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatalf("Open ledger: %v", err)
	}
	shipment, err := ledger.GetShipmentByTracking(ctx, receipt.TrackingNumber)
	if err != nil || shipment == nil {
		t.Fatalf("GetShipmentByTracking=%#v err=%v", shipment, err)
	}
	if shipment.RequestHash == "" || shipment.Status != "created" || shipment.LabelPath != receipt.LabelPath {
		t.Fatalf("shipment ledger=%#v", shipment)
	}
	ledger.Close()

	pickupBody := requestMap(t, validSchedulePickupRequest())
	pickupResponse := []byte(`{"transactionId":"pickup-tx","output":{"pickupConfirmationCode":"PICKUP123","location":"EX01","scheduledDate":"2026-09-03","carrierCode":"FDXE"}}`)
	result, err = PersistSuccess(ctx, ActionSchedulePickup, pickupBody, pickupResponse, PersistOptions{OperationID: "pickup-op", PickupPreflight: "verified", PickupPreflightCutoff: "16:00", PickupPreflightAccessStart: "09:00"})
	if err != nil {
		t.Fatalf("PersistSuccess(pickup): %v", err)
	}
	pickupReceipt := result.(PickupReceipt)
	if pickupReceipt.ConfirmationNumber != "PICKUP123" || pickupReceipt.LocationCode != "EX01" {
		t.Fatalf("pickup receipt=%#v", pickupReceipt)
	}
	ledger, err = store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	pickup, err := ledger.GetPickupByConfirmation(ctx, "PICKUP123")
	if err != nil || pickup == nil {
		t.Fatalf("pickup ledger=%#v err=%v", pickup, err)
	}
	if pickup.TransactionID != "pickup-tx" || pickup.CutoffTime != "16:00" || pickup.Status != "scheduled" {
		t.Fatalf("pickup ledger=%#v", pickup)
	}
	ledger.Close()
}

func TestResolvePickupCancellationFromLedgerAndLegacy(t *testing.T) {
	t.Setenv("FEDEX_DATA_DIR", filepath.Join(t.TempDir(), "fedex"))
	ctx := context.Background()
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertPickup(ctx, store.Pickup{OperationID: "pickup-op", ConfirmationNumber: "CONF123", AccountNumber: "123456789", CarrierCode: "FDXE", ScheduledDate: "2026-09-03", LocationCode: "EX01", RequestHash: "hash", Status: "scheduled"}); err != nil {
		t.Fatal(err)
	}
	ledger.Close()

	resolved, err := ResolvePickupCancellation(ctx, map[string]any{"pickupConfirmationCode": "CONF123"}, "")
	if err != nil {
		t.Fatalf("ResolvePickupCancellation: %v", err)
	}
	fields, err := ExtractOperationalFields(ActionCancelPickup, resolved.Body)
	if err != nil {
		t.Fatal(err)
	}
	if fields.AccountNumber != "123456789" || fields.CarrierCode != "FDXE" || fields.LocationCode != "EX01" {
		t.Fatalf("resolved fields=%#v", fields)
	}
	if _, err := ResolvePickupCancellation(ctx, map[string]any{"pickupConfirmationCode": "CONF123", "carrierCode": "FDXG"}, ""); err == nil {
		t.Fatal("conflicting carrier was accepted")
	}

	legacy := map[string]any{"associatedAccountNumber": map[string]any{"value": "999"}, "pickupConfirmationCode": "LEGACY", "carrierCode": "FDXG", "scheduledDate": "2026-09-03"}
	if _, err := ResolvePickupCancellation(ctx, legacy, ""); err == nil {
		t.Fatal("unmatched legacy pickup was accepted without a reason")
	}
	if _, err := ResolvePickupCancellation(ctx, legacy, "customer requested legacy cancellation"); err != nil {
		t.Fatalf("legacy pickup with reason: %v", err)
	}
}

func requestMap(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body
}
