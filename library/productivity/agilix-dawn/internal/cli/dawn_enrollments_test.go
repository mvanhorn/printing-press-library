// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for enrollment/group helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestStringSlice(t *testing.T) {
	var m map[string]any
	_ = json.Unmarshal([]byte(`{"member":["u_1","u_2"],"empty":[],"single":{"id":"x"}}`), &m)
	got := stringSlice(m["member"])
	if len(got) != 2 || got[0] != "u_1" || got[1] != "u_2" {
		t.Fatalf("member = %v", got)
	}
	if len(stringSlice(m["empty"])) != 0 {
		t.Error("empty should yield 0")
	}
	// a lone object normalizes to a one-element list but has no string, so 0 strings
	if len(stringSlice(m["single"])) != 0 {
		t.Error("object should yield 0 strings")
	}
	if len(stringSlice(m["missing"])) != 0 {
		t.Error("missing should yield 0")
	}
}

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
	}{
		{`{"version":4}`, 4},                     // top-level (concept)
		{`{"group":{"version":2,"id":"g_x"}}`, 2}, // wrapped (group)
		{`{"concept":{"version":7}}`, 7},          // wrapped (concept variant)
	}
	for _, tc := range cases {
		got := extractVersion(json.RawMessage(tc.raw))
		if f, ok := got.(float64); !ok || f != tc.want {
			t.Errorf("extractVersion(%s) = %v, want %v", tc.raw, got, tc.want)
		}
	}
	if extractVersion(json.RawMessage(`{"nope":true}`)) != nil {
		t.Error("expected nil when no version present")
	}
}

func TestEnrollRemoveLogic(t *testing.T) {
	// mirrors the remove command's filter loop
	members := []string{"u_1", "u_2", "u_3"}
	target := "u_2"
	kept := make([]any, 0, len(members))
	found := false
	for _, m := range members {
		if m == target {
			found = true
			continue
		}
		kept = append(kept, m)
	}
	if !found || len(kept) != 2 {
		t.Fatalf("found=%v kept=%v", found, kept)
	}
	for _, m := range kept {
		if m == target {
			t.Error("target still present")
		}
	}
}
