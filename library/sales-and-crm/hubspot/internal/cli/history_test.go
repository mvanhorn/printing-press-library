// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

// helperOpenStoreWithMutations seeds a temp store with three audit-log rows
// and returns the store + a cmd context-ready cobra command for direct
// queryHistory drives.
func helperOpenStoreWithMutations(t *testing.T, commands ...string) *store.Store {
	t.Helper()
	if len(commands) == 0 {
		commands = []string{"sync", "contacts bulk-update", "import"}
	}
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	for _, cmd := range commands {
		s.LogMutation(ctx, cmd, []string{"--db", dbPath}, 0, -1, "cli", 25*time.Millisecond)
		// Avoid identical CURRENT_TIMESTAMP timestamps for sort stability.
		time.Sleep(1100 * time.Millisecond)
	}
	return s
}

func TestQueryHistory_AllRecent(t *testing.T) {
	s := helperOpenStoreWithMutations(t)
	cmd := newHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	cutoff := time.Now().Add(-1 * time.Hour)
	rows, err := queryHistory(cmd, s, cutoff, nil, 100)
	if err != nil {
		t.Fatalf("queryHistory: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 history rows, got %d (%+v)", len(rows), rows)
	}
	// Most-recent first: last seeded ("import") should lead.
	if rows[0].Command != "import" {
		t.Errorf("expected newest row command=import, got %q", rows[0].Command)
	}
}

func TestQueryHistory_KindFilter(t *testing.T) {
	s := helperOpenStoreWithMutations(t)
	cmd := newHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	cutoff := time.Now().Add(-1 * time.Hour)
	rows, err := queryHistory(cmd, s, cutoff, []string{"sync"}, 100)
	if err != nil {
		t.Fatalf("queryHistory: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 sync row, got %d (%+v)", len(rows), rows)
	}
	if rows[0].Command != "sync" {
		t.Errorf("kind filter leaked: got command=%q", rows[0].Command)
	}
}

func TestQueryHistory_SinceCutoffExcludes(t *testing.T) {
	s := helperOpenStoreWithMutations(t, "sync")
	cmd := newHistoryCmd(&rootFlags{})
	cmd.SetContext(context.Background())
	// Cutoff in the FUTURE — no rows can be newer.
	cutoff := time.Now().Add(1 * time.Hour)
	rows, err := queryHistory(cmd, s, cutoff, nil, 100)
	if err != nil {
		t.Fatalf("queryHistory: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows past future cutoff, got %d (%+v)", len(rows), rows)
	}
}

func TestParseSinceArg_Relative(t *testing.T) {
	got, err := parseSinceArg("1h")
	if err != nil {
		t.Fatalf("parseSinceArg 1h: %v", err)
	}
	delta := time.Since(got)
	if delta < 50*time.Minute || delta > 70*time.Minute {
		t.Errorf("expected 1h-ago cutoff, got delta=%s", delta)
	}
}

func TestParseSinceArg_RFC3339(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err := parseSinceArg(want.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("parseSinceArg RFC3339: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestSanitizeArgs_RedactsCredentials(t *testing.T) {
	got := sanitizeArgs([]string{"sync", "--token", "abc123", "--api-key=xyz", "--secret=top"})
	want := []string{"sync", "--token", "[REDACTED]", "--api-key=[REDACTED]", "--secret=[REDACTED]"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
