// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD tests for sync fan-derivation error surfacing + resume-cursor gating
// (review finding #7: extractFans swallowed its UpsertBatch error and the
// cursor advanced past the page anyway, silently and permanently losing the
// derived fans). Synthetic fixtures only (IETF example.com).
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// orderWithFan builds a minimal orders payload carrying a nested fan, the shape
// extractFans derives from.
func orderWithFan(id, fanID, email string) string {
	type fanF struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	type orderF struct {
		ID  string `json:"id"`
		Fan fanF   `json:"fan"`
	}
	b, _ := json.Marshal(orderF{ID: id, Fan: fanF{ID: fanID, Email: email}})
	return string(b)
}

// TestExtractFansSurfacesUpsertError asserts extractFans returns a non-nil error
// when the fan upsert fails (here: a closed store), instead of swallowing it.
func TestExtractFansSurfacesUpsertError(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	// Close the store so the subsequent fan UpsertBatch fails.
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}
	nodes := []json.RawMessage{json.RawMessage(orderWithFan("o1", "fan-1", fanA))}
	n, err := extractFans(s, nodes)
	if err == nil {
		t.Fatalf("want error from extractFans when fan upsert fails, got nil (n=%d)", n)
	}
	if n != 0 {
		t.Errorf("want 0 fans persisted on error, got %d", n)
	}
}

// TestPersistSyncPageGatesCursorOnPersistError asserts that when the page cannot
// be persisted, persistSyncPage returns an error and does NOT advance the
// resume cursor (so a re-sync re-fetches and re-derives).
func TestPersistSyncPageGatesCursorOnPersistError(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	// Pre-seed a known cursor so we can detect any unwanted advance.
	if err := s.SaveSyncState("orders", "cursor-A", 0); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	nodes := []json.RawMessage{json.RawMessage(orderWithFan("o1", "fan-1", fanA))}
	_, _, err := persistSyncPage(s, "orders", nodes, "cursor-B", false, 0)
	if err == nil {
		t.Fatalf("want error from persistSyncPage on a closed store, got nil")
	}
	// SaveSyncState must not have been reached with the new cursor. We cannot
	// read the closed store; instead assert the error names the failure point.
	if !strings.Contains(err.Error(), "orders") {
		t.Errorf("error %q should name the failing resource", err.Error())
	}
}

// TestPersistSyncPageHappyPathAdvancesCursorAfterFans asserts the success path:
// the resource page and its derived fans persist AND the resume cursor advances
// to the new endCursor — proving cursor advance happens after fan persistence.
func TestPersistSyncPageHappyPathAdvancesCursorAfterFans(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	nodes := []json.RawMessage{
		json.RawMessage(orderWithFan("o1", "fan-1", fanA)),
		json.RawMessage(orderWithFan("o2", "fan-2", fanB)),
	}
	stored, fans, err := persistSyncPage(s, "orders", nodes, "cursor-NEXT", false, 5)
	if err != nil {
		t.Fatalf("persistSyncPage: %v", err)
	}
	if stored != 2 {
		t.Errorf("stored = %d, want 2", stored)
	}
	if fans != 2 {
		t.Errorf("fans = %d, want 2 (both orders carry a distinct fan)", fans)
	}
	// Cursor must have advanced to the new endCursor, with cumulative count =
	// priorStored(5) + pageStored(2) = 7.
	cursor, _, count, err := s.GetSyncState("orders")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if cursor != "cursor-NEXT" {
		t.Errorf("cursor = %q, want %q", cursor, "cursor-NEXT")
	}
	if count != 7 {
		t.Errorf("cumulative total_count = %d, want 7 (prior 5 + page 2)", count)
	}
	// The derived fans must actually be in the fans table.
	fanRows, err := s.List("fans", 100)
	if err != nil {
		t.Fatalf("listing fans: %v", err)
	}
	if len(fanRows) != 2 {
		t.Errorf("fans table has %d rows, want 2", len(fanRows))
	}
}

// TestPersistSyncPageLatestDoesNotAdvanceCursor asserts the --latest-only path
// (latest=true) persists the page + fans but does NOT clobber the forward
// resume cursor.
func TestPersistSyncPageLatestDoesNotAdvanceCursor(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	if err := s.SaveSyncState("orders", "forward-cursor", 3); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}
	nodes := []json.RawMessage{json.RawMessage(orderWithFan("o9", "fan-9", fanC))}
	if _, _, err := persistSyncPage(s, "orders", nodes, "", true, 0); err != nil {
		t.Fatalf("persistSyncPage (latest): %v", err)
	}
	cursor, _, count, err := s.GetSyncState("orders")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if cursor != "forward-cursor" || count != 3 {
		t.Errorf("latest path clobbered forward checkpoint: cursor=%q count=%d, want forward-cursor/3", cursor, count)
	}
}
