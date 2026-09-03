// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

func TestCancelPickupResolvesIdentifiersFromLedger(t *testing.T) {
	var calls int
	var sent map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &sent); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionId":"cancel-pickup","output":{"pickupConfirmationCode":"PU123","cancelConfirmationMessage":"Pickup request has been cancelled"}}`))
	}))
	defer api.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", filepath.Join(t.TempDir(), "fedex"))
	setMCPTestAuth(t, api.URL)
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertPickup(context.Background(), store.Pickup{OperationID: "pickup-create", ConfirmationNumber: "PU123", AccountNumber: "123456789", CarrierCode: "FDXG", ScheduledDate: "2026-09-04", RequestHash: "pickup-hash", Status: "scheduled"}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["cancel_pickup"]
	request := map[string]any{"pickupConfirmationCode": "PU123"}
	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request}))
	if err != nil || preview == nil || preview.IsError {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	if calls != 0 {
		t.Fatalf("preview calls=%d", calls)
	}
	var pending struct {
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, preview)), &pending); err != nil {
		t.Fatal(err)
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
	if calls != 1 || sent["carrierCode"] != "FDXG" || sent["scheduledDate"] != "2026-09-04" {
		t.Fatalf("calls=%d sent=%#v", calls, sent)
	}
	account, _ := sent["associatedAccountNumber"].(map[string]any)
	if account["value"] != "123456789" {
		t.Fatalf("resolved account=%#v", account)
	}
	ledger, _ = store.Open(store.DefaultPath())
	pickup, err := ledger.GetPickupByConfirmation(context.Background(), "PU123")
	_ = ledger.Close()
	if err != nil || pickup == nil || pickup.Status != "cancelled" || pickup.CancellationTransactionID != "cancel-pickup" {
		t.Fatalf("pickup=%+v err=%v", pickup, err)
	}
}
