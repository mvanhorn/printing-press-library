// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/store"
)

func TestSearchRequiresExplicitZIP(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--data-source", "local", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--zip is required") {
		t.Fatalf("search error = %v, want required ZIP", err)
	}
}

func TestSearchLocalUsesSelectedMarket(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []json.RawMessage{
		json.RawMessage(`{"id":"same","name":"East coffee","_sync_postal_code":"10001","_sync_locale":"en-us"}`),
		json.RawMessage(`{"id":"same","name":"West coffee","_sync_postal_code":"94105","_sync_locale":"en-us"}`),
	} {
		if _, _, err := db.UpsertBatch("flyers", []json.RawMessage{item}); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--type", "flyers", "--zip", "10001", "--data-source", "local", "--json", "--db", dbPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("local search: %v (%s)", err, output.String())
	}
	if !strings.Contains(output.String(), "East coffee") || strings.Contains(output.String(), "West coffee") {
		t.Fatalf("local market search = %s", output.String())
	}
}
