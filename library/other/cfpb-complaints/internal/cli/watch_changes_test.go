// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/internal/store"
)

// TestNovelWatchChangesHelpWires smoke-tests that the watch changes command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWatchChangesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"watch", "changes", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch changes --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "changes"} {
		if !strings.Contains(help, want) {
			t.Fatalf("watch changes --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLoadWatchObservationFallsBackToLegacyKey(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "watch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	legacyKey := legacyWatchObservationKey("ACME", "", "", "30d")
	want := json.RawMessage(`{"legacy":{"product":"Credit card","issue":"Fees"}}`)
	if err := db.Upsert("cfpb-complaint-watch", legacyKey, want); err != nil {
		t.Fatal(err)
	}
	key := watchObservationKey("ACME", "", "", "30d", 100)
	got, baseline, migratedLegacy, err := loadWatchObservation(db, key, legacyKey)
	if err != nil {
		t.Fatal(err)
	}
	if baseline {
		t.Fatal("legacy snapshot was treated as a new baseline")
	}
	if !migratedLegacy {
		t.Fatal("legacy snapshot source was not reported")
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot = %s, want %s", got, want)
	}
	next := json.RawMessage(`{"current":{"product":"Mortgage","issue":"Closing"}}`)
	if err := persistWatchObservation(db, key, legacyKey, next, migratedLegacy); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get("cfpb-complaint-watch", legacyKey); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("legacy snapshot remains after migration: %v", err)
	}
	stored, err := db.Get("cfpb-complaint-watch", key)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(next) {
		t.Fatalf("migrated snapshot = %s, want %s", stored, next)
	}
}

func TestWatchObservationKeyIncludesLimit(t *testing.T) {
	left := watchObservationKey("Company", "Product", "NY", "30d", 20)
	right := watchObservationKey("Company", "Product", "NY", "30d", 100)
	if left == right {
		t.Fatal("different bounded samples shared one snapshot key")
	}
}
