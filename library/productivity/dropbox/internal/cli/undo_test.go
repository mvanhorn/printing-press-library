// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// TestNovelUndoHelpWires smoke-tests that the undo command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUndoHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"undo", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("undo --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "undo"} {
		if !strings.Contains(help, want) {
			t.Fatalf("undo --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestUndoRefusesUndoOfBatch(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "undo-batch", UndoOf: "original", Status: "complete"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err = runRead(t, "undo", "undo-batch", "--db", dbPath, "--json")
	if err == nil || !strings.Contains(err.Error(), "undo batch") {
		t.Fatalf("undo of undo accepted: %v", err)
	}
}

func TestUndoYesRefusesDogfoodWithoutRequests(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "undo", "batch-id", "--yes", "--json")
	var refusal struct {
		Refused bool `json:"refused"`
	}
	if jsonErr := json.Unmarshal(data, &refusal); err != nil || jsonErr != nil || requests != 0 || !refusal.Refused {
		t.Fatalf("undo: %v, requests=%d, output=%s", err, requests, data)
	}
}
