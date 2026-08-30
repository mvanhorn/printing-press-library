// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/store"
)

// TestSearchHintsWhenCatalogPartial guards against search silently treating a
// partial catalog sync as complete. Every other local-read command (family,
// recent, rack, updates, catalog completeness) already warns via
// hintIfCatalogIncomplete when the sync cursor is "partial"; search was the
// one command that skipped the check entirely.
func TestSearchHintsWhenCatalogPartial(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(t.Context(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	doc := extron.Doc{Title: "MAV Plus 328", URL: "https://extron.com/mav-plus-328", Category: "Matrix Switchers"}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	if err := db.Upsert(catalogResource, doc.URL, data); err != nil {
		t.Fatalf("seeding catalog row: %v", err)
	}
	if err := db.SaveSyncState(catalogResource, "partial", 1); err != nil {
		t.Fatalf("saving partial sync state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "MAV", "--db", dbPath, "--data-source", "local"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search error = %v", err)
	}

	if got := out.String(); !strings.Contains(got, "catalog is partial") {
		t.Fatalf("search did not warn about the partial catalog; output:\n%s", got)
	}
}
