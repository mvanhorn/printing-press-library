// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestAnnotateHacktivityProgramRowsEnvelope(t *testing.T) {
	raw := json.RawMessage(`{"pagination":{"page":1},"items":[{"date":"2026-07-06","status":"new","bug_type":{"slug":"cwe-79"},"hunter":{"slug":"h"}}]}`)
	got := annotateHacktivityProgramRows(raw, "program-one")
	var rows []map[string]any
	if err := json.Unmarshal(got, &rows); err != nil {
		t.Fatalf("unmarshal annotated rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if slug := stringField(rows[0], "program.slug"); slug != "program-one" {
		t.Fatalf("program.slug = %q, want program-one", slug)
	}
	if slug := stringField(rows[0], "program_slug"); slug != "program-one" {
		t.Fatalf("program_slug = %q, want program-one", slug)
	}
}

func TestFilterHacktivityProgramRows(t *testing.T) {
	raw := json.RawMessage(`[
		{"date":"2026-07-06","program":{"slug":"keep"}},
		{"date":"2026-07-06","program":{"slug":"drop"}}
	]`)
	got := filterHacktivityProgramRows(raw, "keep")
	var rows []map[string]any
	if err := json.Unmarshal(got, &rows); err != nil {
		t.Fatalf("unmarshal filtered rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if slug := stringField(rows[0], "program.slug"); slug != "keep" {
		t.Fatalf("program.slug = %q, want keep", slug)
	}
}

func TestCWEFromHacktivityBugType(t *testing.T) {
	row := map[string]any{
		"bug_type": map[string]any{
			"name": "Insecure Direct Object Reference (IDOR) (CWE-639)",
			"slug": "insecure-direct-object-reference-idor-cwe-639",
		},
	}
	if got := cweFromReport(row); got != "CWE-639" {
		t.Fatalf("cweFromReport = %q, want CWE-639", got)
	}
}
