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

func TestCancelShipmentPersistsExactCancellationState(t *testing.T) {
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionId":"cancel-ship","output":{"cancelledShipment":true}}`))
	}))
	defer api.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", filepath.Join(t.TempDir(), "fedex"))
	setMCPTestAuth(t, api.URL)
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.InsertShipment(context.Background(), store.Shipment{TrackingNumber: "123456789012", Account: "123456789", ServiceType: "FEDEX_GROUND", ShipperCountry: "US", Status: "created"}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["cancel_shipment"]
	request := map[string]any{"accountNumber": map[string]any{"value": "123456789"}, "trackingNumber": "123456789012", "senderCountryCode": "US", "deletionControl": "DELETE_ALL_PACKAGES"}
	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request}))
	if err != nil || preview == nil || preview.IsError {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	var pending struct {
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
		Review             struct {
			TrackingNumber string `json:"tracking_number"`
			DeletionMode   string `json:"deletion_control"`
		} `json:"review"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, preview)), &pending); err != nil {
		t.Fatal(err)
	}
	if pending.Review.TrackingNumber != "123456789012" || pending.Review.DeletionMode != "DELETE_ALL_PACKAGES" {
		t.Fatalf("review=%+v", pending.Review)
	}
	confirmed, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request":             request,
		"confirm":             true,
		"operation_id":        pending.OperationID,
		"confirmation_digest": pending.ConfirmationDigest,
	}))
	if err != nil || confirmed == nil || confirmed.IsError {
		t.Fatalf("confirmed=%#v err=%v", confirmed, err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	ledger, _ = store.Open(store.DefaultPath())
	shipment, err := ledger.GetShipmentByTracking(context.Background(), "123456789012")
	_ = ledger.Close()
	if err != nil || shipment == nil || shipment.Status != "cancelled" || shipment.CancellationTransactionID != "cancel-ship" {
		t.Fatalf("shipment=%+v err=%v", shipment, err)
	}
}
