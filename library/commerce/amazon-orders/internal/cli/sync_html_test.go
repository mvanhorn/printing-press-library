package cli

import (
	"encoding/json"
	"testing"
)

func TestRejectHTMLSyncPayload(t *testing.T) {
	err := rejectHTMLSyncPayload(json.RawMessage(" \n<!DOCTYPE html><html><head><title>Your Orders</title></head></html>"))
	if err == nil {
		t.Fatal("expected an HTML response to be rejected before it reaches the JSON store")
	}
}

func TestRejectHTMLSyncPayloadAllowsJSON(t *testing.T) {
	if err := rejectHTMLSyncPayload(json.RawMessage(`[{"orderId":"111-2222222-3333333"}]`)); err != nil {
		t.Fatalf("expected JSON payload to be accepted: %v", err)
	}
}
