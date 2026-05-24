// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func TestParseInteractionsArray(t *testing.T) {
	raw := json.RawMessage(`[
	  {"type":"sent","timestamp":"2026-05-10T08:00:00Z"},
	  {"type":"delivered","timestamp":"2026-05-10T08:00:05Z"}
	]`)
	events := parseInteractions(raw)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "sent" || events[1].Type != "delivered" {
		t.Fatalf("wrong order/type: %+v", events)
	}
}

func TestParseInteractionsWrapped(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"type":"failed","timestamp":"2026-05-10T08:00:10Z","reason":"carrier_blocked"}]}`)
	events := parseInteractions(raw)
	if len(events) != 1 || events[0].Type != "failed" || events[0].Reason != "carrier_blocked" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestIsTerminalFailure(t *testing.T) {
	for _, v := range []string{"failed", "undelivered", "REJECTED", "Expired"} {
		if !isTerminalFailure(v) {
			t.Errorf("%q should be terminal failure", v)
		}
	}
	for _, v := range []string{"sent", "delivered", "queued", "read"} {
		if isTerminalFailure(v) {
			t.Errorf("%q should NOT be terminal failure", v)
		}
	}
}

func TestIsTerminalSuccess(t *testing.T) {
	if !isTerminalSuccess("delivered") {
		t.Error("delivered should be terminal success")
	}
	if !isTerminalSuccess("READ") {
		t.Error("read (any case) should be terminal success")
	}
	if isTerminalSuccess("sent") {
		t.Error("sent is not terminal")
	}
}
