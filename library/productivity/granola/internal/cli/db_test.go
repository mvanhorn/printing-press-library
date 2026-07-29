// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store"
)

// TestDBSchemaListsRealColumns pins the contract the command exists for:
// the schema it prints is the schema scripts will hit with sqlite3. The
// two columns asserted here are the two that past sessions guessed wrong
// (meetings.row_source misread as "source", folders.title as "name").
func TestDBSchemaListsRealColumns(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := granola.EnsureSchema(context.Background(), s.DB()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	s.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newDBSchemaCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("db schema: %v", err)
	}

	var got dbSchemaOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not the documented JSON shape: %v (raw=%s)", err, out.String())
	}
	if got.Path != dbPath {
		t.Errorf("path = %q, want %q", got.Path, dbPath)
	}
	cols := map[string]map[string]bool{}
	for _, tb := range got.Tables {
		cols[tb.Name] = map[string]bool{}
		for _, c := range tb.Columns {
			cols[tb.Name][c.Name] = true
		}
	}
	if !cols["meetings"]["row_source"] {
		t.Errorf("meetings.row_source missing from schema output; tables=%v", cols)
	}
	if cols["meetings"]["source"] {
		t.Errorf("meetings.source should not exist — the real column is row_source")
	}
	if !cols["folders"]["title"] {
		t.Errorf("folders.title missing from schema output")
	}
	if cols["folders"]["name"] {
		t.Errorf("folders.name should not exist — the real column is title")
	}
}

// TestDBSchemaMissingStoreExitsNotFound covers the no-store path: a clear
// pointer at sync, not a bare os.Stat error.
func TestDBSchemaMissingStoreExitsNotFound(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{asJSON: true}
	cmd := newDBSchemaCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--db", filepath.Join(t.TempDir(), "absent.db")})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a missing store")
	}
	if ExitCode(err) == 1 {
		t.Errorf("missing store should carry the not-found exit code, got generic 1 (err=%v)", err)
	}
}
