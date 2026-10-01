// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/weathernews/internal/store"
)

func TestWorkflowArchiveFailsClosedWithoutResources(t *testing.T) {
	cases := [][]string{
		{"workflow", "archive"},
		{"--json", "workflow", "archive"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := RootCmd()
			var stdout, stderr strings.Builder
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected non-zero exit\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
			if !strings.Contains(err.Error(), "no archiveable resources") {
				t.Fatalf("error = %v", err)
			}
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, "Archived 0 items") {
				t.Fatalf("claimed empty success\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestWorkflowStatusMigrationKeepsLearnings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO search_learnings (query_pattern, resource_id, action, source) VALUES ('京都', 'place-1', 'boost', 'taught')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(fmt.Sprintf("PRAGMA user_version = %d", store.StoreSchemaVersion-1)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"workflow", "status", "--db", dbPath})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected a schema migration error")
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "delete") {
		t.Fatalf("migration advice suggests deleting the store: %s", msg)
	}
	wantCmd := fmt.Sprintf("weathernews-pp-cli sync --db %q", dbPath)
	if !strings.Contains(msg, wantCmd) || !strings.Contains(msg, "in place") {
		t.Fatalf("migration advice = %s, want command %s", msg, wantCmd)
	}

	kept, err := store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer kept.Close()
	var n int
	if err := kept.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_id = 'place-1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("taught learning rows = %d, want 1", n)
	}
}
