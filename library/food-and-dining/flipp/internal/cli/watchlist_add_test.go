// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNovelWatchlistAddHelpWires smoke-tests that the watchlist add command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWatchlistAddHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"watchlist", "add", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watchlist add --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "add"} {
		if !strings.Contains(help, want) {
			t.Fatalf("watchlist add --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestUpsertWatchEntryReplacesSameQueryZipLocale(t *testing.T) {
	oldTime := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	entries := []watchEntry{{
		Query:       "milk",
		TargetPrice: 3.50,
		Zip:         "85050",
		Locale:      "en-us",
		AddedAt:     oldTime,
	}}

	updated, replaced := upsertWatchEntry(entries, watchEntry{
		Query:       "Milk",
		TargetPrice: 2.99,
		Zip:         "85050",
		Locale:      "en-us",
		AddedAt:     newTime,
	})
	if !replaced {
		t.Fatal("expected existing watch entry to be replaced")
	}
	if len(updated) != 1 {
		t.Fatalf("len(updated) = %d, want 1", len(updated))
	}
	if got, want := updated[0].TargetPrice, 2.99; got != want {
		t.Fatalf("target price = %.2f, want %.2f", got, want)
	}
	if !updated[0].AddedAt.Equal(newTime) {
		t.Fatalf("added_at = %s, want %s", updated[0].AddedAt, newTime)
	}
}

func TestLoadWatchEntriesRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadWatchEntries(path); err == nil {
		t.Fatal("expected malformed watchlist JSON to return an error")
	} else if !strings.Contains(err.Error(), "invalid watchlist JSON") {
		t.Fatalf("error = %q, want invalid watchlist JSON context", err)
	}
}
