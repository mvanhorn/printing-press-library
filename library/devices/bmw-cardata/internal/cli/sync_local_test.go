// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"
	"github.com/spf13/cobra"
)

func TestLocalDataSourceDoesNotClearSyncCursors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("BMW_CARDATA_BASE_URL", server.URL)
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "access", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		newCommand func(*rootFlags) *cobra.Command
		args       []string
	}{
		{"sync-full", newSyncCmd, []string{"--resources", "customers", "--full"}},
		{"sync-latest-only", newSyncCmd, []string{"--resources", "customers", "--latest-only"}},
		{"workflow-archive-full", newWorkflowArchiveCmd, []string{"--full"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "data.db")
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SaveSyncState("customers", "saved-cursor", 42); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			cmd := tc.newCommand(&rootFlags{dataSource: "local", configPath: cfg.Path, timeout: time.Second})
			cmd.SetArgs(append([]string{"--db", dbPath}, tc.args...))
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--data-source local") {
				t.Fatalf("expected local-only refusal, got %v", err)
			}
			db, err = store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cursor, _, count, err := db.GetSyncState("customers")
			if err != nil || cursor != "saved-cursor" || count != 42 {
				t.Fatalf("local-only command changed sync state: cursor=%q count=%d err=%v", cursor, count, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("local-only sync/archive made %d provider requests", calls.Load())
	}
}

func TestLocalDataSourceRefusesImportAndTailBeforeWork(t *testing.T) {
	for _, tc := range []struct {
		name       string
		newCommand func(*rootFlags) *cobra.Command
		args       []string
	}{
		{"import", newImportCmd, []string{"customers", "--input", filepath.Join(t.TempDir(), "missing.jsonl")}},
		{"tail", newTailCmd, []string{"customers", "--follow=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCommand(&rootFlags{dataSource: "local"})
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--data-source local") {
				t.Fatalf("expected local-only refusal before reading input or polling, got %v", err)
			}
		})
	}
}
