// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseProductPage(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantItems int
		wantNext  string
	}{
		{
			name:      "envelope with page_info cursor",
			raw:       `{"data":[{"id":1},{"id":2}],"page_info":{"next_cursor":"abc"}}`,
			wantItems: 2,
			wantNext:  "abc",
		},
		{
			name:      "envelope with null cursor (last page)",
			raw:       `{"data":[{"id":1}],"page_info":{"next_cursor":null}}`,
			wantItems: 1,
			wantNext:  "",
		},
		{
			name:      "top-level next_cursor fallback",
			raw:       `{"data":[{"id":1}],"next_cursor":"xyz"}`,
			wantItems: 1,
			wantNext:  "xyz",
		},
		{
			name:      "bare array fallback",
			raw:       `[{"id":1},{"id":2},{"id":3}]`,
			wantItems: 3,
			wantNext:  "",
		},
		{
			name:      "empty data",
			raw:       `{"data":[],"page_info":{"next_cursor":null}}`,
			wantItems: 0,
			wantNext:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, next := parseProductPage(json.RawMessage(tc.raw))
			if len(items) != tc.wantItems {
				t.Errorf("items = %d, want %d", len(items), tc.wantItems)
			}
			if next != tc.wantNext {
				t.Errorf("next = %q, want %q", next, tc.wantNext)
			}
		})
	}
}

func TestProductSKU(t *testing.T) {
	cases := []struct {
		name string
		obj  map[string]any
		want string
	}{
		{"prefers unique_merchant_sku", map[string]any{"unique_merchant_sku": "ABC-1", "id": "999"}, "ABC-1"},
		{"falls back to id", map[string]any{"id": "12345"}, "12345"},
		{"empty when neither present", map[string]any{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := productSKU(tc.obj); got != tc.want {
				t.Errorf("productSKU = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCoalesceFloat(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if got := coalesceFloat(nil, f(2.0), f(3.0)); got == nil || *got != 2.0 {
		t.Errorf("expected first non-nil 2.0, got %v", got)
	}
	if got := coalesceFloat(nil, nil); got != nil {
		t.Errorf("expected nil when all nil, got %v", *got)
	}
}

func TestListRetailerIDsParsing(t *testing.T) {
	// listRetailerIDs unmarshals a top-level array; verify the id extraction
	// shape without a network call by replicating the same unmarshal it does.
	raw := `[{"id":"walmart","name":"Walmart"},{"id":"target","name":"Target"},{"name":"no-id"}]`
	var arr []map[string]any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ids := make([]string, 0, len(arr))
	for _, r := range arr {
		if id, ok := r["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) != 2 || ids[0] != "walmart" || ids[1] != "target" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}

func TestNovelIndexDryRun(t *testing.T) {
	dbPath := newTempDBPath(t)
	flags := &rootFlags{dryRun: true}
	cmd := newNovelIndexCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--db", dbPath, "--retailer", "walmart"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "would index") {
		t.Fatalf("expected a 'would index' dry-run line, got %q", got)
	}
}

// newTempDBPath returns a path inside the test temp dir without opening it.
func newTempDBPath(t *testing.T) string {
	t.Helper()
	return t.TempDir() + "/index.db"
}
