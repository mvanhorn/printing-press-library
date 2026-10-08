// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// TestNovelJournalHelpWires smoke-tests that the journal command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelJournalHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"journal", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("journal --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "journal"} {
		if !strings.Contains(help, want) {
			t.Fatalf("journal --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestJournalRestoreDaysRemaining(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "old", CreatedAt: at, Status: "complete", RestoreDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddDropboxJournalOp(context.Background(), store.DropboxJournalOp{BatchID: "old", Seq: 1, Op: "delete", Path: "/A/file", Result: "ok"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	data, err := runRead(t, "journal", "old", "--db", dbPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result journalList
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Batches) != 1 || result.Batches[0].RestoreDaysRemaining == nil || *result.Batches[0].RestoreDaysRemaining != 20 {
		t.Fatalf("journal: %+v", result)
	}
}

func TestJournalHappyArgsListsBatches(t *testing.T) {
	cmd := newNovelJournalCmd(&rootFlags{})
	if got := cmd.Annotations["pp:happy-args"]; got != "--limit=5" {
		t.Fatalf("journal happy args = %q, want --limit=5", got)
	}
	if !strings.Contains(cmd.Example, "journal 20261006-153012-abcd --ops") {
		t.Fatalf("batch-id example missing: %q", cmd.Example)
	}
}

func TestJournalShowsUnknownJobAndOperation(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "b", Status: "partial"}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddDropboxJournalOp(context.Background(), store.DropboxJournalOp{BatchID: "b", Seq: 1, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Result: "unknown", AsyncJobID: "job-42", Error: "poll failed"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	cmd := RootCmd()
	cmd.SetArgs([]string{"journal", "b", "--ops", "--db", dbPath, "--human-friendly"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown=1") || !strings.Contains(out.String(), "job=job-42") || !strings.Contains(out.String(), "poll failed") {
		t.Fatalf("output=%s", out.String())
	}
}
