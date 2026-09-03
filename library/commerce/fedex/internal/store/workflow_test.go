// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestWorkflowLedgerPersistsShipmentAndPickupRecoveryFields(t *testing.T) {
	state, err := Open(filepath.Join(t.TempDir(), "fedex.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer state.Close()
	ctx := context.Background()

	if _, err := state.InsertShipment(ctx, Shipment{
		TrackingNumber: "TRACK-1",
		ServiceType:    "FEDEX_GROUND",
		TransactionID:  "tx-create",
		RequestHash:    "sha256:create",
		Status:         "created",
	}); err != nil {
		t.Fatalf("InsertShipment: %v", err)
	}
	updated, err := state.UpdateShipmentCancellation(ctx, "TRACK-1", "cancelled", "tx-cancel")
	if err != nil || !updated {
		t.Fatalf("UpdateShipmentCancellation updated=%v err=%v", updated, err)
	}
	shipment, err := state.GetShipmentByTracking(ctx, "TRACK-1")
	if err != nil {
		t.Fatalf("GetShipmentByTracking: %v", err)
	}
	if shipment.Status != "cancelled" || shipment.CancellationStatus != "cancelled" || shipment.CancellationTransactionID != "tx-cancel" || shipment.CancelledAt == nil {
		t.Fatalf("shipment cancellation fields not persisted: %#v", shipment)
	}

	pickup := Pickup{
		OperationID:        "operation-1",
		ConfirmationNumber: "PICKUP-1",
		AccountNumber:      "123456789",
		CarrierCode:        "FDXG",
		ScheduledDate:      "2026-09-03",
		RequestHash:        "sha256:pickup",
		TransactionID:      "tx-pickup",
		Status:             "scheduled",
		PreflightStatus:    "verified",
	}
	if err := state.UpsertPickup(ctx, pickup); err != nil {
		t.Fatalf("UpsertPickup: %v", err)
	}
	stored, err := state.GetPickupByConfirmation(ctx, "PICKUP-1")
	if err != nil {
		t.Fatalf("GetPickupByConfirmation: %v", err)
	}
	if stored == nil || stored.OperationID != pickup.OperationID || stored.TransactionID != pickup.TransactionID {
		t.Fatalf("pickup recovery fields not persisted: %#v", stored)
	}
	updated, err = state.UpdatePickupCancellation(ctx, "PICKUP-1", "cancelled", "tx-pickup-cancel")
	if err != nil || !updated {
		t.Fatalf("UpdatePickupCancellation updated=%v err=%v", updated, err)
	}
	stored, err = state.GetPickupByOperationID(ctx, "operation-1")
	if err != nil {
		t.Fatalf("GetPickupByOperationID: %v", err)
	}
	if stored.Status != "cancelled" || stored.CancellationTransactionID != "tx-pickup-cancel" || stored.CancelledAt == nil {
		t.Fatalf("pickup cancellation fields not persisted: %#v", stored)
	}
}

func TestOpenMigratesLegacyShipmentTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fedex.db")
	state, err := Open(path)
	if err != nil {
		t.Fatalf("Open initial: %v", err)
	}
	if _, err := state.db.Exec("DROP TABLE shipments"); err != nil {
		t.Fatalf("drop shipments: %v", err)
	}
	if _, err := state.db.Exec(`CREATE TABLE shipments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tracking_number TEXT NOT NULL UNIQUE,
		master_tracking_number TEXT,
		account TEXT,
		service_type TEXT NOT NULL,
		packaging_type TEXT,
		shipper_name TEXT,
		shipper_postal TEXT,
		shipper_country TEXT,
		recipient_name TEXT,
		recipient_address TEXT,
		recipient_city TEXT,
		recipient_state TEXT,
		recipient_postal TEXT,
		recipient_country TEXT,
		weight_value REAL,
		weight_units TEXT,
		reference TEXT,
		net_charge_amount REAL,
		net_charge_currency TEXT,
		list_charge_amount REAL,
		label_path TEXT,
		raw_response TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy shipments: %v", err)
	}
	if err := state.Close(); err != nil {
		t.Fatalf("close initial: %v", err)
	}

	state, err = Open(path)
	if err != nil {
		t.Fatalf("Open migrated: %v", err)
	}
	defer state.Close()
	for _, column := range []string{"transaction_id", "request_hash", "status", "cancellation_transaction_id", "cancellation_status", "cancelled_at"} {
		var count int
		if err := state.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('shipments') WHERE name = ?", column).Scan(&count); err != nil {
			t.Fatalf("inspect column %s: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("migration did not add column %s", column)
		}
	}
}
