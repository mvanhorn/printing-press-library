// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

// helperOpenStoreWithMeetingHistory seeds an in-temp-dir SQLite store with a
// few property-history rows for a single meeting so the three meetings-history
// command-family tests don't have to repeat the setup.
func helperOpenStoreWithMeetingHistory(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	t1 := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 3, 11, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC)
	entries := []store.PropertyHistoryEntry{
		{Property: "hs_meeting_outcome", Value: "SCHEDULED", Timestamp: t1, Source: "API"},
		{Property: "hs_meeting_outcome", Value: "COMPLETED", Timestamp: t3, Source: "USER"},
		{Property: "hs_meeting_title", Value: "Discovery", Timestamp: t2, Source: "USER"},
	}
	if err := s.UpsertPropertyHistoryBatch(ctx, "meetings", "m1", entries); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func TestQueryMeetingsHistory_NoFilter(t *testing.T) {
	s := helperOpenStoreWithMeetingHistory(t)
	// Drive queryMeetingsHistory directly via a fresh cobra command so
	// cmd.Context() returns the test's context.
	cmd := newNovelMeetingsHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	rows, err := queryMeetingsHistory(cmd, s, "m1", "")
	if err != nil {
		t.Fatalf("queryMeetingsHistory: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 history rows, got %d", len(rows))
	}
	// First row should be the most-recent timestamp (DESC order).
	if rows[0].Property != "hs_meeting_outcome" || rows[0].Value != "COMPLETED" {
		t.Errorf("expected newest row hs_meeting_outcome=COMPLETED, got %+v", rows[0])
	}
}

func TestQueryMeetingsHistory_PropertyFilter(t *testing.T) {
	s := helperOpenStoreWithMeetingHistory(t)
	cmd := newNovelMeetingsHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	rows, err := queryMeetingsHistory(cmd, s, "m1", "hs_meeting_outcome")
	if err != nil {
		t.Fatalf("queryMeetingsHistory: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 outcome rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Property != "hs_meeting_outcome" {
			t.Errorf("filter leaked: got property=%q", r.Property)
		}
	}
}

func TestQueryMeetingsHistory_UnknownMeeting(t *testing.T) {
	s := helperOpenStoreWithMeetingHistory(t)
	cmd := newNovelMeetingsHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	rows, err := queryMeetingsHistory(cmd, s, "does-not-exist", "")
	if err != nil {
		t.Fatalf("queryMeetingsHistory: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for unknown meeting, got %d", len(rows))
	}
}
