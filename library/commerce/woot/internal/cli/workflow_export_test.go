// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/store"
)

func TestWorkflowArchiveSelectsDealsResource(t *testing.T) {
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	flags := rootFlags{dryRun: true, asJSON: true, timeout: time.Second, rateLimit: 0}
	cmd := newWorkflowArchiveCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--db", filepath.Join(dir, "data.db")})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow archive: %v\nstderr: %s", err, stderr.String())
	}
	var output struct {
		ResourcesSynced int `json:"resources_synced"`
		ResourcesTotal  int `json:"resources_total"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode archive output: %v\nstdout: %s", err, stdout.String())
	}
	if output.ResourcesSynced != 1 || output.ResourcesTotal != 1 {
		t.Fatalf("archive resources = %+v, want one synced deal resource", output)
	}
}

func TestWorkflowArchiveFullDryRunPreservesSyncState(t *testing.T) {
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	dbPath := filepath.Join(dir, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.SaveSyncState("deals", "300", 300); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := rootFlags{dryRun: true, asJSON: true, timeout: time.Second, rateLimit: 0}
	cmd := newWorkflowArchiveCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--db", dbPath, "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow archive --full --dry-run: %v", err)
	}

	db, err = store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer db.Close()
	cursor, _, count, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "300" || count != 300 {
		t.Fatalf("sync state after dry run = cursor %q count %d, want cursor 300 count 300", cursor, count)
	}
}

func TestWorkflowArchiveFailedFullRefreshPreservesLastSuccessfulMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"upstream failure"}]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY", "test-key")
	t.Setenv("WOOT_BASE_URL", server.URL)
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.SaveSyncState("deals", "300", 300); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}
	_, before, _, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read initial sync state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := rootFlags{timeout: time.Second, rateLimit: 0}
	cmd := newWorkflowArchiveCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--db", dbPath, "--full"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "upstream failure") {
		t.Fatalf("failed full refresh error = %v", err)
	}

	db, err = store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer db.Close()
	cursor, after, count, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read final sync state: %v", err)
	}
	if cursor != "300" || count != 300 || !after.Equal(before) {
		t.Fatalf("sync state after failed refresh = cursor %q count %d time %s, want cursor 300, count 300, time %s", cursor, count, after, before)
	}
	incomplete, err := db.HasIncompleteSyncContext(context.Background())
	if err != nil {
		t.Fatalf("read incomplete state after failed refresh: %v", err)
	}
	if !incomplete {
		t.Fatal("failed full refresh published the prior incomplete store as ready")
	}
}

func TestWorkflowStatusDoesNotCreateMissingDatabase(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	cmd := newWorkflowStatusCmd(&rootFlags{})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--db", dbPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow status: %v", err)
	}
	if !strings.Contains(stdout.String(), "No archived data") {
		t.Fatalf("status output = %q", stdout.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("missing status database was created: %v", err)
	}
}

func TestExportDealsReadsLocalStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for _, item := range []struct {
		id   string
		data string
	}{
		{id: "one", data: `{"id":"one","title":"First deal","min_price":9.99}`},
		{id: "two", data: `{"id":"two","title":"Second deal","min_price":19.99}`},
	} {
		if err := db.Upsert("deals", item.id, json.RawMessage(item.data)); err != nil {
			t.Fatalf("seed %s: %v", item.id, err)
		}
	}
	if err := db.SaveSyncState("deals", "", 2); err != nil {
		t.Fatalf("save sync state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := rootFlags{maxAge: time.Hour}
	cmd := newExportCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"deals", "--db", dbPath, "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export deals: %v\nstderr: %s", err, stderr.String())
	}
	var output []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode export output: %v\nstdout: %s", err, stdout.String())
	}
	if len(output) != 2 {
		t.Fatalf("exported rows = %d, want 2", len(output))
	}
}

func TestExportRejectsDatabaseAsOutput(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.Upsert("deals", "one", json.RawMessage(`{"id":"one","title":"First deal"}`)); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	for _, output := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		cmd := newExportCmd(&rootFlags{})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"deals", "--db", dbPath, "--output", output})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "must not overwrite") {
			t.Fatalf("export output %q error = %v", output, err)
		}
	}

	db, err = store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("reopen store read-only: %v", err)
	}
	defer db.Close()
	if _, err := db.Get("deals", "one"); err != nil {
		t.Fatalf("database was damaged by rejected export: %v", err)
	}
}

func TestExportRejectsOutputAliasOfDatabase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	alias := filepath.Join(dir, "database-alias")
	if err := os.Link(dbPath, alias); err != nil {
		t.Fatalf("create database alias: %v", err)
	}
	if err := validateExportOutputPath(dbPath, alias); err == nil || !strings.Contains(err.Error(), "SQLite database") {
		t.Fatalf("alias validation error = %v", err)
	}

	db, err = store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer db.Close()
	if _, err := db.Get("deals", "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("database query error = %v", err)
	}
}

func TestExportRejectsDanglingSidecarSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	output := filepath.Join(dir, "export.jsonl")
	if err := os.Symlink(dbPath+"-wal", output); err != nil {
		t.Fatalf("create dangling sidecar symlink: %v", err)
	}
	if err := validateExportOutputPath(dbPath, output); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("dangling sidecar validation error = %v", err)
	}
}
