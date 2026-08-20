// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
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

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"
)

// TestNovelQueueHelpWires smoke-tests that the queue command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelQueueHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"queue", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("queue --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "queue"} {
		if !strings.Contains(help, want) {
			t.Fatalf("queue --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestQueueLocalReadsAreDrainFirstAndNullSafe(t *testing.T) {
	ctx, db := openLocalCommandTestStore(t)
	insertLocalCommandResource(t, ctx, db, "posts", "10", `{
		"id": 10,
		"status": "draft",
		"modified": "2026-07-01T12:00:00",
		"title": {"rendered": "Draft"},
		"author": 11
	}`)
	insertLocalCommandResource(t, ctx, db, "users", "11", `{"id": 11}`)

	rows, err := loadQueueRows(ctx, db, []string{"draft"})
	if err != nil {
		t.Fatalf("loadQueueRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("loadQueueRows() len = %d, want 1", len(rows))
	}
	if rows[0].Date != "" || rows[0].Link != "" || !rows[0].AuthorID.Valid {
		t.Fatalf("loadQueueRows() optional fields = %+v", rows[0])
	}

	authors, err := loadQueueAuthors(ctx, db)
	if err != nil {
		t.Fatalf("loadQueueAuthors() after drained posts error = %v", err)
	}
	if name, ok := authors[11]; !ok || name != "" {
		t.Fatalf("loadQueueAuthors()[11] = %q, %v; want empty present name", name, ok)
	}
}

func TestDaysInState(t *testing.T) {
	now := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want int
	}{
		{name: "three complete days old", at: now.Add(-72 * time.Hour), want: 3},
		{name: "partial past day truncates down", at: now.Add(-47 * time.Hour), want: 1},
		{name: "same instant", at: now, want: 0},
		{name: "partial future day remains future", at: now.Add(12 * time.Hour), want: -1},
		{name: "three future days", at: now.Add(72 * time.Hour), want: -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysInState(now, tt.at); got != tt.want {
				t.Fatalf("daysInState() = %d, want %d", got, tt.want)
			}
		})
	}
}

func openLocalCommandTestStore(t *testing.T) (context.Context, *store.Store) {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "mirror.db"))
	if err != nil {
		t.Fatalf("store.OpenWithContext() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("store.Close() error = %v", err)
		}
	})
	return ctx, db
}

func insertLocalCommandResource(t *testing.T, ctx context.Context, db *store.Store, resourceType, id, data string) {
	t.Helper()
	if _, err := db.DB().ExecContext(ctx,
		`INSERT INTO resources (resource_type, id, data) VALUES (?, ?, ?)`,
		resourceType, id, data,
	); err != nil {
		t.Fatalf("insert %s/%s fixture error = %v", resourceType, id, err)
	}
}
