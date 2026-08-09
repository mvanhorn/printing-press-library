// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/habitica/internal/store"
)

// TestNovelWeekReviewHelpWires smoke-tests that the week review command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWeekReviewHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"week", "review", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("week review --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "review"} {
		if !strings.Contains(help, want) {
			t.Fatalf("week review --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReadHabiticaWeekSnapshotHandlesEmptyTaskMirror(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "habitica.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	snapshot, err := readHabiticaWeekSnapshot(context.Background(), db.DB(), now)
	if err != nil {
		t.Fatalf("read empty snapshot: %v", err)
	}
	if snapshot.Open != 0 || snapshot.Completed != 0 || snapshot.Overdue != 0 {
		t.Fatalf("empty snapshot counts = %+v, want all zero", snapshot)
	}
	if snapshot.CapturedAt != now.Format(time.RFC3339) {
		t.Fatalf("captured_at = %q, want %q", snapshot.CapturedAt, now.Format(time.RFC3339))
	}
}
