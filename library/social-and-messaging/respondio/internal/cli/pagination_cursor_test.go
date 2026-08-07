// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func mustEnvelope(t *testing.T, s string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	return m
}

// TestExtractPaginationFromEnvelope_NumericCursor asserts that the integer
// Respond.io style cursor (pagination.next as a JSON number) is read and
// rendered as a plain integer string.
func TestExtractPaginationFromEnvelope_NumericCursor(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}],"pagination":{"next":12345}}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "12345" || !hasMore {
		t.Fatalf("numeric next: cur=%q hasMore=%v, want %q true", cur, hasMore, "12345")
	}
}

// TestExtractPaginationFromEnvelope_StringCursor keeps the existing string
// cursor behavior working.
func TestExtractPaginationFromEnvelope_StringCursor(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}],"pagination":{"next":"12345"}}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "12345" || !hasMore {
		t.Fatalf("string next: cur=%q hasMore=%v, want %q true", cur, hasMore, "12345")
	}
}

func TestExtractPaginationFromEnvelope_NoPagination(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}]}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "" || hasMore {
		t.Fatalf("no pagination object: cur=%q hasMore=%v, want empty cursor + hasMore=false", cur, hasMore)
	}
}

func TestExtractPaginationFromEnvelope_EmptyPagination(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}],"pagination":{}}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "" || hasMore {
		t.Fatalf("empty pagination object: cur=%q hasMore=%v, want empty cursor + hasMore=false", cur, hasMore)
	}
}

// TestDeterminePaginationDefaults_UsesCursorId asserts every resource (and the
// fallback) reports the cursor parameter the Respond.io spec declares.
func TestDeterminePaginationDefaults_UsesCursorId(t *testing.T) {
	for _, res := range []string{"contact", "message", "space", "unknown-resource"} {
		if got := determinePaginationDefaults(res).cursorParam; got != "cursorId" {
			t.Fatalf("resource %q cursorParam = %q, want %q", res, got, "cursorId")
		}
	}
}

func TestResourceSupportsPagination_Contact(t *testing.T) {
	if !resourceSupportsPagination("contact") {
		t.Fatal("resourceSupportsPagination(contact) = false, want true")
	}
}
