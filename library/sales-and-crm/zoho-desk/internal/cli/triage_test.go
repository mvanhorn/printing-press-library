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

// TestNovelTriageHelpWires smoke-tests that the triage command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTriageHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"triage", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("triage --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "triage"} {
		if !strings.Contains(help, want) {
			t.Fatalf("triage --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelTriageBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	now := time.Now()
	// Ticket 1: unassigned + overdue + high priority -> should rank first.
	db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "ticketNumber": "101", "subject": "hot", "status": "Open",
		"priority": "High", "assigneeId": "", "dueDate": zohoTime(now.Add(-3 * time.Hour)),
	}))
	// Ticket 2: assigned, future due, low priority -> low score.
	db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "ticketNumber": "102", "subject": "calm", "status": "Open",
		"priority": "Low", "assigneeId": "A1", "dueDate": zohoTime(now.Add(48 * time.Hour)),
	}))
	db.Upsert("tickets", "3", mustJSON(t, map[string]any{"id": "3", "status": "Closed", "priority": "High"}))
	db.Close()

	out := runTranscendCmd(t, "triage", "--db", dbPath, "--json")
	var view struct {
		Count          int `json:"count"`
		ScannedTickets int `json:"scanned_tickets"`
		Tickets        []struct {
			ID      string   `json:"id"`
			Score   float64  `json:"score"`
			Reasons []string `json:"reasons"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.Count != 2 {
		t.Fatalf("expected 2 open tickets scored, got %d: %s", view.Count, out)
	}
	if view.Tickets[0].ID != "1" {
		t.Fatalf("expected unassigned overdue ticket 1 ranked first, got %+v", view.Tickets)
	}
	joined := strings.Join(view.Tickets[0].Reasons, ",")
	if !strings.Contains(joined, "unassigned") || !strings.Contains(joined, "overdue") {
		t.Errorf("expected reasons to include unassigned and overdue, got %v", view.Tickets[0].Reasons)
	}
}
