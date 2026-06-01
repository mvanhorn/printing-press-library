// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestDroppedFilterParams verifies that only non-empty, non-pagination params
// count as dropped filters on a local read. Regression guard for the silent
// filter-drop bug: a local read that ignored --friend-id returned the whole
// store with no in-band signal, and the prior "any param present" check would
// have flagged every read (limit/offset carry defaults) — making the signal
// useless.
func TestDroppedFilterParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]string
		want   []string
	}{
		{
			name:   "only pagination defaults -> nothing dropped",
			params: map[string]string{"limit": "20", "offset": "0"},
			want:   []string{},
		},
		{
			name:   "unset filters arrive empty -> nothing dropped",
			params: map[string]string{"friend_id": "", "group_id": "", "limit": "20", "offset": "0"},
			want:   []string{},
		},
		{
			name:   "real friend filter -> flagged",
			params: map[string]string{"friend_id": "107803424", "group_id": "", "limit": "20", "offset": "0"},
			want:   []string{"friend_id"},
		},
		{
			name:   "multiple real filters -> sorted",
			params: map[string]string{"group_id": "28194161", "friend_id": "107803424", "dated_after": "2025-01-01", "limit": "20"},
			want:   []string{"dated_after", "friend_id", "group_id"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := droppedFilterParams(tc.params)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("droppedFilterParams = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWrapWithProvenance_UnscopedMeta verifies the unscoped signal is serialized
// into the meta envelope (the in-band signal agents consume) when set, and
// omitted otherwise.
func TestWrapWithProvenance_UnscopedMeta(t *testing.T) {
	data := json.RawMessage(`[{"id":1}]`)

	// Unscoped local read: meta must carry unscoped + unapplied_params.
	out, err := wrapWithProvenance(data, DataProvenance{
		Source:          "local",
		Unscoped:        true,
		UnappliedParams: []string{"friend_id"},
	})
	if err != nil {
		t.Fatalf("wrapWithProvenance: %v", err)
	}
	var env struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Meta["unscoped"] != true {
		t.Fatalf("meta.unscoped = %v, want true", env.Meta["unscoped"])
	}
	if got, ok := env.Meta["unapplied_params"].([]any); !ok || len(got) != 1 || got[0] != "friend_id" {
		t.Fatalf("meta.unapplied_params = %v, want [friend_id]", env.Meta["unapplied_params"])
	}

	// Scoped/normal read: neither key should be present. Use a fresh struct —
	// json.Unmarshal merges into an existing non-nil map rather than replacing it.
	out2, err := wrapWithProvenance(data, DataProvenance{Source: "local"})
	if err != nil {
		t.Fatalf("wrapWithProvenance: %v", err)
	}
	var env2 struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out2, &env2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := env2.Meta["unscoped"]; ok {
		t.Fatalf("meta.unscoped present on a scoped read, want absent")
	}
	if _, ok := env2.Meta["unapplied_params"]; ok {
		t.Fatalf("meta.unapplied_params present on a scoped read, want absent")
	}
}
