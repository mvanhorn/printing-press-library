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

// mustJSON marshals v to json.RawMessage for store seeding in tests.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	return b
}

// zohoTime formats t as a Zoho-style ISO-8601 timestamp.
func zohoTime(tm time.Time) string {
	return tm.UTC().Format("2006-01-02T15:04:05.000Z")
}

// runTranscendCmd runs the root command with the given args and returns stdout.
func runTranscendCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v error = %v\noutput:\n%s", args, err, out.String())
	}
	return out.String()
}

// TestNovelSlaRadarHelpWires smoke-tests that the sla-radar command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSlaRadarHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"sla-radar", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sla-radar --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "sla-radar"} {
		if !strings.Contains(help, want) {
			t.Fatalf("sla-radar --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelSlaRadarBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	now := time.Now()
	if err := db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "ticketNumber": "101", "subject": "due soon", "status": "Open",
		"priority": "High", "assigneeId": "A1", "dueDate": zohoTime(now.Add(2 * time.Hour)),
	})); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "ticketNumber": "102", "subject": "far future", "status": "Open",
		"priority": "Low", "dueDate": zohoTime(now.Add(100 * time.Hour)),
	})); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("tickets", "3", mustJSON(t, map[string]any{
		"id": "3", "ticketNumber": "103", "subject": "closed", "status": "Closed",
		"dueDate": zohoTime(now.Add(1 * time.Hour)),
	})); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := runTranscendCmd(t, "sla-radar", "--within", "24h", "--db", dbPath, "--json")
	var view struct {
		Count          int `json:"count"`
		ScannedTickets int `json:"scanned_tickets"`
		Tickets        []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.ScannedTickets != 3 {
		t.Errorf("scanned_tickets = %d, want 3", view.ScannedTickets)
	}
	if view.Count != 1 || len(view.Tickets) != 1 || view.Tickets[0].ID != "1" {
		t.Fatalf("expected only ticket 1 due soon, got %+v", view.Tickets)
	}
}
