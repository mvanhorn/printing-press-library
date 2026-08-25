// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package appscrape

import (
	"strings"
	"testing"
)

// extractPayloadArray must locate the Next.js Flight value chunk, walk
// brackets honoring strings + escapes, and return the closed [...] slice.
// Regressions here silently break every `app <slug>` scrape.
func TestExtractPayloadArray_Basic(t *testing.T) {
	in := `prefix-junk[{"value":[{"id":"flow1"}]}]suffix`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, `[{"value":`) || !strings.HasSuffix(got, `}]`) {
		t.Fatalf("not a balanced slice: %s", got)
	}
}

// Bracket balance must survive nested objects + arrays.
func TestExtractPayloadArray_Nested(t *testing.T) {
	in := `noise[{"value":[{"a":[1,2,{"b":[3]}]}]}]more`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "[") != strings.Count(got, "]") {
		t.Fatalf("unbalanced: %s", got)
	}
}

// Strings containing closing brackets must not advance the depth counter.
func TestExtractPayloadArray_StringWithBrackets(t *testing.T) {
	in := `[{"value":[{"label":"close ] in string"}]}]`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `close ] in string`) {
		t.Fatalf("string truncated: %s", got)
	}
}

// Missing payload pattern must error, not silently emit garbage.
func TestExtractPayloadArray_Missing(t *testing.T) {
	if _, err := extractPayloadArray(`nothing useful here`); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindString(t *testing.T) {
	rows := []map[string]any{{"x": 1}, {"appName": "Stripe"}}
	if got := findString(rows, "appName", "app_name"); got != "Stripe" {
		t.Fatalf("got %q", got)
	}
	if got := findString(rows, "nope"); got != "" {
		t.Fatalf("got %q; want empty", got)
	}
}
