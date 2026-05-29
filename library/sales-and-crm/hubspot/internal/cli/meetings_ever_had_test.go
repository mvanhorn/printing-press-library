// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

func TestQueryMeetingsEverHad_FindsMeetingInWindow(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Three meetings: m1 was SCHEDULED in window, m2 was SCHEDULED before window,
	// m3 never SCHEDULED.
	may1 := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	apr10 := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	may15 := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)

	if err := s.UpsertPropertyHistoryBatch(ctx, "meetings", "m1", []store.PropertyHistoryEntry{
		{Property: "hs_meeting_outcome", Value: "SCHEDULED", Timestamp: may1},
		{Property: "hs_meeting_outcome", Value: "COMPLETED", Timestamp: may15},
	}); err != nil {
		t.Fatalf("seed m1: %v", err)
	}
	if err := s.UpsertPropertyHistoryBatch(ctx, "meetings", "m2", []store.PropertyHistoryEntry{
		{Property: "hs_meeting_outcome", Value: "SCHEDULED", Timestamp: apr10},
	}); err != nil {
		t.Fatalf("seed m2: %v", err)
	}
	if err := s.UpsertPropertyHistoryBatch(ctx, "meetings", "m3", []store.PropertyHistoryEntry{
		{Property: "hs_meeting_outcome", Value: "NO_SHOW", Timestamp: may1},
	}); err != nil {
		t.Fatalf("seed m3: %v", err)
	}

	cmd := newNovelMeetingsEverHadCmd(&rootFlags{})
	cmd.SetContext(ctx)
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)

	rows, err := queryMeetingsEverHad(cmd, s, "hs_meeting_outcome", "SCHEDULED", from, to, cliutil.FilterExpr{})
	if err != nil {
		t.Fatalf("queryMeetingsEverHad: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 match (m1), got %d: %+v", len(rows), rows)
	}
	if rows[0].MeetingID != "m1" {
		t.Errorf("expected m1, got %q", rows[0].MeetingID)
	}
}

func TestParseDateWindow_Defaults(t *testing.T) {
	from, to, err := parseDateWindow("", "")
	if err != nil {
		t.Fatalf("parseDateWindow: %v", err)
	}
	if !from.Equal(time.Unix(0, 0)) {
		t.Errorf("default from should be epoch, got %v", from)
	}
	if to.Before(time.Now().Add(-time.Minute)) || to.After(time.Now().Add(time.Minute)) {
		t.Errorf("default to should be ~now, got %v", to)
	}
}

func TestParseDateWindow_RejectsInvertedRange(t *testing.T) {
	_, _, err := parseDateWindow("2026-05-31", "2026-05-01")
	if err == nil {
		t.Fatal("expected error on inverted range")
	}
}

func TestParseDateOrTimestamp(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"2026-05-01", false},
		{"2026-05-01T09:00:00Z", false},
		{"not-a-date", true},
	}
	for _, c := range cases {
		_, err := parseDateOrTimestamp(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseDateOrTimestamp(%q) wantErr=%v gotErr=%v", c.in, c.wantErr, err)
		}
	}
}
