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

// TestNovelMorningHelpWires smoke-tests that the morning command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMorningHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"morning", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("morning --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "morning"} {
		if !strings.Contains(help, want) {
			t.Fatalf("morning --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelMorningBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Upsert("agents", "A1", mustJSON(t, map[string]any{"id": "A1", "firstName": "Ada", "lastName": "L"}))
	now := time.Now()
	// Open ticket due within 24h, assigned to A1, recently modified.
	db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "ticketNumber": "101", "subject": "due soon", "status": "Open", "priority": "High",
		"assigneeId": "A1", "dueDate": zohoTime(now.Add(3 * time.Hour)), "modifiedTime": zohoTime(now.Add(-1 * time.Hour)),
	}))
	db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "ticketNumber": "102", "subject": "far", "status": "Open", "assigneeId": "A1",
		"dueDate": zohoTime(now.Add(200 * time.Hour)),
	}))
	db.Close()

	out := runTranscendCmd(t, "morning", "--db", dbPath, "--json")
	var view struct {
		BreachForecast struct {
			Within string `json:"within"`
			Count  int    `json:"count"`
			Top    []struct {
				ID string `json:"id"`
			} `json:"top"`
		} `json:"breachForecast"`
		AgentLoad struct {
			Median     float64 `json:"median"`
			Overloaded []struct {
				AgentID string `json:"agentId"`
			} `json:"overloaded"`
		} `json:"agentLoad"`
		RecentChanges struct {
			Since string `json:"since"`
			Count int    `json:"count"`
		} `json:"recentChanges"`
		ScannedTickets int `json:"scanned_tickets"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.ScannedTickets != 2 {
		t.Errorf("scanned_tickets=%d, want 2", view.ScannedTickets)
	}
	if view.BreachForecast.Within != "24h" || view.BreachForecast.Count != 1 || len(view.BreachForecast.Top) != 1 {
		t.Fatalf("breachForecast unexpected: %+v", view.BreachForecast)
	}
	if view.RecentChanges.Since != "12h" || view.RecentChanges.Count != 1 {
		t.Errorf("recentChanges unexpected: %+v", view.RecentChanges)
	}
	if len(view.AgentLoad.Overloaded) != 1 || view.AgentLoad.Overloaded[0].AgentID != "A1" {
		t.Errorf("expected A1 overloaded, got %+v", view.AgentLoad.Overloaded)
	}
}
