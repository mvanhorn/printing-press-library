// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuditListUsesDedicatedKeyAndPaginatesSerially(t *testing.T) {
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer audit_test_key" {
			t.Errorf("Authorization = %q", got)
		}
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		if r.URL.Query().Get("action") != "workspace" ||
			r.URL.Query().Get("occurred_after") != "2026-09-01T00:00:00Z" ||
			r.URL.Query().Get("occurred_before") != "2026-09-21T04:30:00Z" {
			t.Errorf("filters = %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("cursor") == "audit_next" {
			_, _ = w.Write([]byte(`{"events":[{"id":"evt_2","action":"note.viewed"}],"hasMore":false,"cursor":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"events":[{"id":"evt_1","action":"note.created"}],"hasMore":true,"cursor":"audit_next"}`))
	}))
	defer srv.Close()
	t.Setenv("GRANOLA_BASE_URL", srv.URL)
	t.Setenv("GRANOLA_API_KEY", "regular_key_must_not_be_used")
	t.Setenv("GRANOLA_AUDIT_API_KEY", "audit_test_key")

	out, _, err := runCLISplit(t, "audit", "list", "--action", "workspace",
		"--occurred-after", "2026-09-01", "--occurred-before", "2026-09-20T23:30:00-05:00",
		"--all", "--json")
	if err != nil {
		t.Fatalf("audit list: %v (out=%s)", err, out)
	}
	var events []map[string]any
	if err := json.Unmarshal([]byte(out), &events); err != nil {
		t.Fatalf("decode output: %v (%q)", err, out)
	}
	if len(events) != 2 || len(cursors) != 2 || cursors[1] != "audit_next" {
		t.Fatalf("events=%v cursors=%v", events, cursors)
	}
}

func TestAuditListRejectsInvalidTimeBeforeRequest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("GRANOLA_BASE_URL", srv.URL)
	t.Setenv("GRANOLA_AUDIT_API_KEY", "audit_test_key")

	_, _, err := runCLISplit(t, "audit", "list", "--occurred-after", "not-a-date")
	if err == nil {
		t.Fatal("expected invalid time error")
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestNormalizeAuditTimestampEnforcesOneYearRetentionBoundary(t *testing.T) {
	now := time.Date(2026, time.September, 20, 15, 30, 0, 0, time.UTC)
	got, err := normalizeAuditTimestampAt("occurred-after", "2025-09-20", now)
	if err != nil {
		t.Fatalf("boundary timestamp rejected: %v", err)
	}
	if got != "2025-09-20T00:00:00Z" {
		t.Fatalf("normalized boundary = %q", got)
	}
	if _, err := normalizeAuditTimestampAt("occurred-after", "2025-09-19T23:59:59Z", now); err == nil {
		t.Fatal("expected timestamp outside one-year retention window to fail")
	}
	if _, err := normalizeAuditTimestampAt("occurred-after", "2025-09-20T15:29:59Z", now); err == nil {
		t.Fatal("expected RFC3339 timestamp before the precise retention cutoff to fail")
	}
}

func TestAuditListRequiresDedicatedKey(t *testing.T) {
	t.Setenv("GRANOLA_AUDIT_API_KEY", "")
	_, _, err := runCLISplit(t, "audit", "list")
	if err == nil {
		t.Fatal("expected missing audit key error")
	}
}
