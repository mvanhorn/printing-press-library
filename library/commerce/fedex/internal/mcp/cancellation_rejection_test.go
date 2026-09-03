// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

func TestExplicitCancellationFailureIsRejectedNotCancelled(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionId":"tx-reject","output":{"cancelledShipment":false},"unrelated":{"success":true,"message":"recipient data"}}`))
	}))
	defer api.Close()
	dataDir := filepath.Join(t.TempDir(), "fedex")
	t.Setenv("FEDEX_DATA_DIR", dataDir)
	setMCPTestAuth(t, api.URL)
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.InsertShipment(context.Background(), store.Shipment{TrackingNumber: "TRACK-REJECT", Account: "123456789", ServiceType: "FEDEX_GROUND", Status: "created"}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["cancel_shipment"]
	request := map[string]any{"accountNumber": map[string]any{"value": "123456789"}, "senderCountryCode": "US", "trackingNumber": "TRACK-REJECT", "deletionControl": "DELETE_ALL_PACKAGES"}
	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request}))
	if err != nil || preview == nil || preview.IsError {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	var pending struct {
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, preview)), &pending); err != nil {
		t.Fatal(err)
	}
	result, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request, "confirm": true, "operation_id": pending.OperationID, "confirmation_digest": pending.ConfirmationDigest}))
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("result=%#v err=%v, want rejection", result, err)
	}
	var failure struct {
		ErrorClass      string `json:"error_class"`
		OperationStatus string `json:"operation_status"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, result)), &failure); err != nil {
		t.Fatal(err)
	}
	if failure.ErrorClass != "remote_rejected" || failure.OperationStatus != "rejected" {
		t.Fatalf("failure=%+v", failure)
	}
	ledger, err = store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	shipment, err := ledger.GetShipmentByTracking(context.Background(), "TRACK-REJECT")
	_ = ledger.Close()
	if err != nil || shipment == nil || shipment.Status == "cancelled" || shipment.CancellationStatus != "cancel_rejected" {
		t.Fatalf("shipment=%+v err=%v", shipment, err)
	}
}
