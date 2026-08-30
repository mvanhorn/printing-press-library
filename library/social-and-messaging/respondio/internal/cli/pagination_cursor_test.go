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

// TestExtractPaginationFromEnvelope_URLValuedWrapperCursor covers the live
// Respond.io shape where pagination.next holds a full URL rather than a bare
// token. The cursor lives in that URL's query string, so returning the URL
// itself (which the name-based scan would do, since "next" is a cursor key)
// sends an unusable cursorId and stops the sync after page one.
func TestExtractPaginationFromEnvelope_URLValuedWrapperCursor(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}],"pagination":{"next":"https://api.respond.io/v2/contact/list?limit=100&cursorId=4242"}}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "4242" || !hasMore {
		t.Fatalf("URL next: cur=%q hasMore=%v, want %q true", cur, hasMore, "4242")
	}
}

// TestExtractPaginationFromEnvelope_NullWrapperCursor asserts the terminal
// page (pagination.next explicitly null) stops the walk.
func TestExtractPaginationFromEnvelope_NullWrapperCursor(t *testing.T) {
	env := mustEnvelope(t, `{"items":[{"id":1}],"pagination":{"next":null}}`)
	cur, hasMore := extractPaginationFromEnvelope(env, "cursorId")
	if cur != "" || hasMore {
		t.Fatalf("null next: cur=%q hasMore=%v, want empty cursor + hasMore=false", cur, hasMore)
	}
}

// TestSyncContactBody_CarriesRequiredWildcardFilter pins the body Respond.io
// requires on POST /contact/list: without search, filter, and timezone the API
// answers HTTP 400 ("Please check if filter is provided") and contact sync
// fails outright.
func TestSyncContactBody_CarriesRequiredWildcardFilter(t *testing.T) {
	_, body := splitSyncPostParams("contact", map[string]string{})
	for _, key := range []string{"search", "filter", "timezone"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("contact sync body missing required %q: %v", key, body)
		}
	}
	filter, ok := body["filter"].(map[string]any)
	if !ok {
		t.Fatalf("contact sync filter = %v (%T), want a wildcard object", body["filter"], body["filter"])
	}
	if _, ok := filter["$and"]; !ok {
		t.Fatalf("contact sync filter missing $and wildcard: %v", filter)
	}
}

// TestSyncContactBody_PaginationParamsGoToQuery asserts limit and cursorId are
// sent as query parameters. Respond.io silently ignores them in the POST body,
// which caps every page at the server default and never advances the cursor.
func TestSyncContactBody_PaginationParamsGoToQuery(t *testing.T) {
	query, body := splitSyncPostParams("contact", map[string]string{
		"limit":    "100",
		"cursorId": "7",
		"search":   "acme",
	})
	if query["limit"] != "100" {
		t.Fatalf("limit query param = %q, want %q (query=%v)", query["limit"], "100", query)
	}
	if query["cursorId"] != "7" {
		t.Fatalf("cursorId query param = %q, want %q (query=%v)", query["cursorId"], "7", query)
	}
	for _, key := range []string{"limit", "cursorId"} {
		if _, ok := body[key]; ok {
			t.Fatalf("%q must not be sent in the contact sync body: %v", key, body)
		}
	}
	// Genuine body params still route to the body.
	if body["search"] != "acme" {
		t.Fatalf("search body param = %v, want %q", body["search"], "acme")
	}
}
