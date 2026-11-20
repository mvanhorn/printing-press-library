// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"time"
)

// TestNovelReadNextHelpWires smoke-tests that the read-next command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelReadNextHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"read-next", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("read-next --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "read-next"} {
		if !strings.Contains(help, want) {
			t.Fatalf("read-next --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelRankReadNextPrefersFitAndLocation(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	docs := []novelBacklogDoc{
		{ID: "long", Title: "Long", Location: "later", EstMinutes: 90, SavedAt: now},
		{ID: "fit", Title: "Fit", Location: "shortlist", EstMinutes: 25, SavedAt: now},
	}
	items := novelRankReadNext(docs, 30*time.Minute, "shortlist", "", 1, now)
	if len(items) != 1 || items[0].ID != "fit" {
		t.Fatalf("top item = %+v, want fit", items)
	}
	if !strings.Contains(items[0].Reason, "fits available time") {
		t.Fatalf("reason = %q, want fit reason", items[0].Reason)
	}
}

func TestNovelRankReadNextAvoidStarted(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	docs := []novelBacklogDoc{
		{ID: "started", Title: "Started", EstMinutes: 10, ReadingProgress: 0.5, SavedAt: now},
		{ID: "unread", Title: "Unread", EstMinutes: 10, SavedAt: now},
	}
	items := novelRankReadNext(docs, 15*time.Minute, "", "started", 5, now)
	if len(items) != 1 || items[0].ID != "unread" {
		t.Fatalf("items = %+v, want started item excluded", items)
	}
}

func TestSliceDReadNextLengthAndPreferenceNote(t *testing.T) {
	sliceDStore(t, "UPDATE documents SET word_count=NULL,reading_time=NULL WHERE id='doc-2'")
	var items []novelReadNextItem
	sliceDView(t, runNovelCommand(t, "read-next", "--have", "30m", "--prefer", "missing"), &items)
	if len(items) != 2 {
		t.Fatalf("%+v", items)
	}
	sliceDView(t, runNovelCommand(t, "read-next", "--have", "30m", "--include-unknown-length"), &items)
	if len(items) != 3 {
		t.Fatalf("%+v", items)
	}
	for _, item := range items {
		if item.ID == "doc-2" && (item.Minutes != nil || item.LengthKnown) {
			t.Fatalf("%+v", item)
		}
	}
	// The hint is independently observable on stderr.
	cmd := RootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"read-next", "--json", "--prefer", "missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "matched no candidate") {
		t.Fatal(stderr.String())
	}
}
func TestSliceDReadNextDeterministicTies(t *testing.T) {
	sliceDStore(t,
		"UPDATE documents SET saved_at='2020-01-01T00:00:00Z', created_at=NULL, reading_progress=0, reading_time=NULL, word_count=238",
		"UPDATE documents SET saved_at='2020-02-01T00:00:00Z' WHERE id='doc-3'",
		"UPDATE documents SET reading_progress=0.8 WHERE id='doc-1'",
		"UPDATE documents SET reading_progress=0.2 WHERE id='doc-2'",
	)
	cmd := newNovelReadNextCmd(&rootFlags{})
	docs, err := novelReadDocuments(cmd)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	items := novelRankReadNext(docs, 0, "", "", 0, now)
	// Started bonus ties doc-1 and doc-2; higher progress breaks their tie.
	if items[0].ID != "doc-1" || items[1].ID != "doc-2" || items[2].ID != "doc-3" {
		t.Fatalf("%+v", items)
	}
	// Clamp recency at zero and remove bonuses to expose saved_at and ID ties.
	for i := range docs {
		docs[i].ReadingProgress = 0
	}
	items = novelRankReadNext(docs, 0, "", "", 0, now)
	if items[0].ID != "doc-3" || items[1].ID != "doc-1" || items[2].ID != "doc-2" {
		t.Fatalf("%+v", items)
	}
	for i, j := 0, len(docs)-1; i < j; i, j = i+1, j-1 {
		docs[i], docs[j] = docs[j], docs[i]
	}
	again := novelRankReadNext(docs, 0, "", "", 0, now)
	for i := range items {
		if items[i].ID != again[i].ID {
			t.Fatal("input order changed rank")
		}
	}
}

func TestSliceDReadNextPreferredLocationBreaksScoreTie(t *testing.T) {
	sliceDStore(t)
	cmd := newNovelReadNextCmd(&rootFlags{})
	docs, err := novelReadDocuments(cmd)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	a, b := docs[0], docs[1]
	a.ID = "preferred"
	a.Location = "shortlist"
	a.SavedAt = now.Add(-365 * 24 * time.Hour)
	a.CreatedAt = time.Time{}
	a.ReadingProgress = 0
	a.EstMinutes = 10
	b.ID = "other"
	b.Location = "later"
	b.SavedAt = now
	b.CreatedAt = time.Time{}
	b.ReadingProgress = 0.5
	b.EstMinutes = 1
	items := novelRankReadNext([]novelBacklogDoc{b, a}, 0, "shortlist", "", 0, now)
	if items[0].Score != items[1].Score || items[0].ID != "preferred" {
		t.Fatalf("%+v", items)
	}
	items = novelRankReadNext([]novelBacklogDoc{b, a}, 0, "shortlist", "started", 0, now)
	if len(items) != 1 || items[0].ID != "preferred" {
		t.Fatalf("%+v", items)
	}
}

func TestE3ReadNextArray(t *testing.T) {
	sliceDStore(t, "UPDATE documents SET word_count=NULL,reading_time=NULL WHERE id='doc-2'")
	for _, mode := range []string{"--json", "--agent"} {
		for _, extra := range [][]string{nil, {"--have", "30m"}, {"--prefer", "missing"}, {"--have", "30m", "--include-unknown-length"}, {"--select", "id"}, {"--db-path", filepath.Join(t.TempDir(), "missing.db")}, {"--quiet"}, {"--csv"}} {
			cmd := RootCmd()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"read-next", mode}, extra...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
				t.Fatalf("array required: %s", out.String())
			}
			if !strings.Contains(stderr.String(), "unknown_length_excluded=") {
				t.Fatal(stderr.String())
			}
		}
	}
}
