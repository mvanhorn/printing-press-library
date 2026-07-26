// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
)

// seedMessages builds a throwaway store holding a few messages and returns its path.
func seedMessages(t *testing.T, texts ...string) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening temp store: %v", err)
	}
	defer db.Close()

	for i, text := range texts {
		ts := "170000000" + string(rune('0'+i)) + ".000000"
		raw, err := json.Marshal(map[string]any{
			"type": "message",
			"user": "U00000000",
			"ts":   ts,
			"text": text,
		})
		if err != nil {
			t.Fatalf("marshalling message: %v", err)
		}
		if err := db.UpsertMessage("C00000000", "U00000000", ts, "", text, 0, "", raw); err != nil {
			t.Fatalf("seeding message %d: %v", i, err)
		}
	}
	return dbPath
}

// runSearch drives the real search command so the test covers the command wiring,
// not just the store helper. That distinction is the whole point: SearchMessages
// was always correct, it simply had no callers, so a test of the store alone would
// have passed against the broken build.
//
// The root command is constructed inline in Execute() with no exported
// constructor, so this builds the search subcommand directly rather than
// refactoring shared code for a test.
func runSearch(t *testing.T, dbPath string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	flags := &rootFlags{asJSON: true, dataSource: "local", noInput: true, yes: true}
	cmd := newSearchCmd(flags)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--db", dbPath))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search failed: %v (output: %s)", err, out.String())
	}
	return out.String()
}

// The regression this guards: sync populates messages and messages_fts, but local
// search never queried them, so a term present in the index returned "No results"
// with exit 0 — a silent wrong answer rather than an error.
func TestLocalSearchFindsMessages(t *testing.T) {
	dbPath := seedMessages(t,
		"deploy the payment service tonight",
		"the renewal quote needs review",
		"unrelated chatter about lunch",
	)

	t.Run("all types finds a message", func(t *testing.T) {
		got := runSearch(t, dbPath, "renewal")
		if !strings.Contains(got, "renewal quote needs review") {
			t.Errorf("expected the seeded message in output, got: %s", got)
		}
	})

	t.Run("explicit --type messages finds it too", func(t *testing.T) {
		got := runSearch(t, dbPath, "payment", "--type", "messages")
		if !strings.Contains(got, "payment service") {
			t.Errorf("expected the seeded message in output, got: %s", got)
		}
	})

	t.Run("absent term still reports nothing", func(t *testing.T) {
		got := runSearch(t, dbPath, "zzqqxxabsentterm")
		if strings.Contains(got, "deploy") || strings.Contains(got, "renewal") {
			t.Errorf("absent term should match nothing, got: %s", got)
		}
	})

	t.Run("limit is respected across aggregated results", func(t *testing.T) {
		got := runSearch(t, dbPath, "the", "--limit", "1")
		// "the" appears in two seeded messages; only one may survive the limit.
		n := strings.Count(got, `"type": "message"`) + strings.Count(got, `"type":"message"`)
		if n > 1 {
			t.Errorf("expected at most 1 result under --limit 1, counted %d in: %s", n, got)
		}
	})
}
