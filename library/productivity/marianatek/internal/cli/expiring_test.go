// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestExpiringRecordIDHandlesFlatAndWrappedRecords(t *testing.T) {
	cases := []struct {
		name string
		rec  map[string]any
		want string
	}{
		{
			name: "jsonapi wrapped",
			rec:  map[string]any{"data": map[string]any{"id": "credit-1"}},
			want: "credit-1",
		},
		{
			name: "flat",
			rec:  map[string]any{"id": "membership-1"},
			want: "membership-1",
		},
		{
			name: "malformed wrapped falls back to flat",
			rec:  map[string]any{"data": "not an object", "id": "flat-1"},
			want: "flat-1",
		},
		{
			name: "missing",
			rec:  map[string]any{"data": "not an object"},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expiringRecordID(tc.rec); got != tc.want {
				t.Fatalf("expiringRecordID() = %q, want %q", got, tc.want)
			}
		})
	}
}
