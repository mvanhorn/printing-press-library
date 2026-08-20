// Copyright 2026 Mherzog4 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/lda-gov/internal/store"
	"github.com/spf13/cobra"
)

func newLDANovelTestDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "lda.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test store: %v", err)
	}
	return dbPath
}

func seedLDANovelRecord(t *testing.T, dbPath, resource, id, raw string) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open test store for seed: %v", err)
	}
	defer db.Close()
	if err := db.Upsert(resource, id, json.RawMessage(raw)); err != nil {
		t.Fatalf("seed %s/%s: %v", resource, id, err)
	}
}

func seedLDANovelSyncState(t *testing.T, dbPath, resource string) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open test store for sync state: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(
		`INSERT INTO sync_state(resource_type, last_synced_at, total_count) VALUES (?, ?, ?)`,
		resource, time.Now(), 1,
	); err != nil {
		t.Fatalf("seed sync state for %s: %v", resource, err)
	}
}

func runLDANovelRows(t *testing.T, cmd *cobra.Command, args ...string) []map[string]any {
	t.Helper()
	rows, _ := runLDANovelRowsWithStderr(t, cmd, args...)
	return rows
}

func runLDANovelRowsWithStderr(t *testing.T, cmd *cobra.Command, args ...string) ([]map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %s %v: %v stderr=%s", cmd.Name(), args, err, stderr.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("decode stdout as rows: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	return rows, stderr.String()
}

func requireLDANovelRow(t *testing.T, rows []map[string]any, key string, want any) map[string]any {
	t.Helper()
	for _, row := range rows {
		if row[key] == want {
			return row
		}
	}
	t.Fatalf("no row with %s=%v in %#v", key, want, rows)
	return nil
}
