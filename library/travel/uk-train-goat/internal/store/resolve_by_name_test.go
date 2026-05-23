// Copyright 2026 ahujasachin92. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestResolveByName_DedupAndInjectionGuard covers the two surfaces of
// ResolveByName that PATCH(upstream cli-printing-press#1249) actually
// touched: the multi-field dedup path (rewritten to slices.Contains
// inside an IIFE so rows.Close is deferred per field) and the SQL-
// injection guard that skips fields failing safeIdentPattern.
func TestResolveByName_DedupAndInjectionGuard(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Two stations. The first has the same value in `name` and `crs_code`
	// so a multi-field call must dedupe to one ID. The second exists only
	// to ensure the dedupe isn't an artifact of a single-row table.
	items := []json.RawMessage{
		json.RawMessage(`{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"EUS","crs_code":"EUS"}`),
		json.RawMessage(`{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","name":"King's Cross","crs_code":"KGX"}`),
	}
	if _, _, err := s.UpsertBatch("station", items); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		fields     []string
		wantID     string
		wantErrSub string
	}{
		{
			name:   "uuid passthrough returns input unchanged",
			input:  "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			fields: []string{"name"},
			wantID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		{
			name:   "single field match resolves to the row id",
			input:  "King's Cross",
			fields: []string{"name"},
			wantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		},
		{
			name:   "multi-field dedup — same row matches both fields, single id returned",
			input:  "EUS",
			fields: []string{"name", "crs_code"},
			wantID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		{
			name:       "no match returns not-found error mentioning sync",
			input:      "Paddington",
			fields:     []string{"name", "crs_code"},
			wantErrSub: "not found",
		},
		{
			name:       "unsafe field identifier is silently skipped (SQL-injection guard)",
			input:      "EUS",
			fields:     []string{"name; DROP TABLE resources"},
			wantErrSub: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ResolveByName("station", tt.input, tt.fields...)
			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (id=%q)", tt.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantID {
				t.Fatalf("want id %q, got %q", tt.wantID, got)
			}
		})
	}

	// Sanity: the unsafe-identifier case must not have dropped the table.
	if _, err := s.ResolveByName("station", "King's Cross", "name"); err != nil {
		t.Fatalf("post-injection probe failed — resources table may be damaged: %v", err)
	}
}
