// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
)

// TestNovelSinceHelpWires smoke-tests that the since command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSinceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"since", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("since --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "since"} {
		if !strings.Contains(help, want) {
			t.Fatalf("since --help missing %q in output:\n%s", want, help)
		}
	}
	if !strings.Contains(help, "since <duration>") {
		t.Errorf("since --help missing 'since <duration>' usage line:\n%s", help)
	}
}

func TestNovelSinceBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	now := time.Now()
	db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "ticketNumber": "101", "subject": "recent", "status": "Open",
		"modifiedTime": zohoTime(now.Add(-1 * time.Hour)),
	}))
	db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "ticketNumber": "102", "subject": "old", "status": "Open",
		"modifiedTime": zohoTime(now.Add(-100 * time.Hour)),
	}))
	db.Close()

	out := runTranscendCmd(t, "since", "12h", "--db", dbPath, "--json")
	var view struct {
		Since          string `json:"since"`
		Count          int    `json:"count"`
		ScannedTickets int    `json:"scanned_tickets"`
		Tickets        []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.Since != "12h" || view.ScannedTickets != 2 {
		t.Errorf("since=%q scanned=%d, want 12h/2", view.Since, view.ScannedTickets)
	}
	if view.Count != 1 || len(view.Tickets) != 1 || view.Tickets[0].ID != "1" {
		t.Fatalf("expected only recently modified ticket 1, got %+v", view.Tickets)
	}
}
