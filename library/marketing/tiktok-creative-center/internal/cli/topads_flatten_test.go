// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"encoding/json"
	"testing"
)

func TestFlattenTopAdID_NoExistingID(t *testing.T) {
	item := json.RawMessage(`{"itemInfo":{"itemID":"12345"}}`)
	out := flattenTopAdID(item)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["id"] != "12345" {
		t.Fatalf("id = %v, want 12345", obj["id"])
	}
}

// TestFlattenTopAdID_CanonicalIDWins reproduces the bug where a pre-existing
// top-level "id" that disagrees with itemInfo.itemID (the true canonical
// identity) was trusted as-is, keying the store under the wrong identity.
// The canonical itemInfo.itemID must always win.
func TestFlattenTopAdID_CanonicalIDWins(t *testing.T) {
	item := json.RawMessage(`{"id":"stale-id","itemInfo":{"itemID":"12345"}}`)
	out := flattenTopAdID(item)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["id"] != "12345" {
		t.Fatalf("id = %v, want canonical 12345 (had stale top-level id)", obj["id"])
	}
}

func TestFlattenTopAdID_AlreadyCanonical(t *testing.T) {
	item := json.RawMessage(`{"id":"12345","itemInfo":{"itemID":"12345"}}`)
	out := flattenTopAdID(item)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["id"] != "12345" {
		t.Fatalf("id = %v, want 12345", obj["id"])
	}
}

func TestFlattenTopAdID_NoItemInfo(t *testing.T) {
	item := json.RawMessage(`{"id":"existing"}`)
	out := flattenTopAdID(item)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["id"] != "existing" {
		t.Fatalf("id = %v, want existing preserved", obj["id"])
	}
}
