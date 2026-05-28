// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for buildDoorList — the pure ticket/returns/transfers join behind
// the `door list` command. No store needed; fixtures are raw node payloads.
package cli

import (
	"encoding/json"
	"testing"
)

func rawDoorNodes(ss ...string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(ss))
	for _, s := range ss {
		out = append(out, json.RawMessage(s))
	}
	return out
}

func TestBuildDoorListExcludesReturnedAndMarksTransferred(t *testing.T) {
	tickets := rawDoorNodes(
		`{"id":"t1","code":"AAA","holder":{"firstName":"Ann","lastName":"A","email":"ann@example.com"}}`,
		`{"id":"t2","code":"BBB","claimedAt":"2026-01-01T00:00:00Z","holder":{"firstName":"Bob","lastName":"B","email":"bob@example.com"}}`,
		`{"id":"t3","code":"CCC","holder":{"firstName":"Cy","lastName":"C","email":"cy@example.com"}}`,
		`{"id":"","code":"NOID","holder":{"email":"skip@example.com"}}`, // no ID -> skipped
	)
	returns := rawDoorNodes(`{"ticketId":"t2"}`)                                                  // t2 refunded -> excluded
	transfers := rawDoorNodes(`{"transferredAt":"2026-02-02T00:00:00Z","tickets":[{"id":"t3"}]}`) // t3 transferred

	entries := buildDoorList(tickets, returns, transfers)

	if len(entries) != 2 {
		t.Fatalf("want 2 entries (t1 valid + t3 transferred; t2 returned and empty-id dropped), got %d: %+v", len(entries), entries)
	}
	byID := map[string]doorEntry{}
	for _, e := range entries {
		byID[e.TicketID] = e
	}
	if _, ok := byID["t2"]; ok {
		t.Errorf("returned ticket t2 must be excluded from the door list")
	}
	if _, ok := byID[""]; ok {
		t.Errorf("empty-ID ticket must be skipped")
	}
	t1, ok := byID["t1"]
	if !ok {
		t.Fatalf("valid ticket t1 missing from door list")
	}
	if t1.HolderName != "Ann A" {
		t.Errorf("t1 holder_name = %q, want %q", t1.HolderName, "Ann A")
	}
	if t1.Transferred {
		t.Errorf("t1 must not be marked transferred")
	}
	t3, ok := byID["t3"]
	if !ok {
		t.Fatalf("transferred ticket t3 missing — a transfer must mark, not drop it")
	}
	if !t3.Transferred || t3.TransferredAt != "2026-02-02T00:00:00Z" {
		t.Errorf("t3 transferred=%v at=%q, want true + the transfer timestamp", t3.Transferred, t3.TransferredAt)
	}
}

func TestBuildDoorListEmptyReturnsNonNilSlice(t *testing.T) {
	entries := buildDoorList(nil, nil, nil)
	if entries == nil {
		t.Fatalf("want non-nil empty slice (must render as [] not null), got nil")
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
	if b, _ := json.Marshal(entries); string(b) != "[]" {
		t.Errorf("marshal = %s, want []", b)
	}
}
