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

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
)

// TestNovelAgentLoadHelpWires smoke-tests that the agent-load command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAgentLoadHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"agent-load", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent-load --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "agent-load"} {
		if !strings.Contains(help, want) {
			t.Fatalf("agent-load --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelAgentLoadBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Upsert("agents", "A1", mustJSON(t, map[string]any{"id": "A1", "firstName": "Ada", "lastName": "Lovelace"}))
	db.Upsert("agents", "A2", mustJSON(t, map[string]any{"id": "A2", "firstName": "Bob", "lastName": "Jones"}))
	// A1: 3 open tickets, A2: 1 open ticket, plus a closed one (ignored).
	for i, id := range []string{"1", "2", "3"} {
		_ = i
		db.Upsert("tickets", id, mustJSON(t, map[string]any{
			"id": id, "status": "Open", "priority": "Medium", "assigneeId": "A1",
		}))
	}
	db.Upsert("tickets", "4", mustJSON(t, map[string]any{"id": "4", "status": "Open", "priority": "Low", "assigneeId": "A2"}))
	db.Upsert("tickets", "5", mustJSON(t, map[string]any{"id": "5", "status": "Closed", "assigneeId": "A1"}))
	db.Close()

	out := runTranscendCmd(t, "agent-load", "--db", dbPath, "--json")
	var view struct {
		Count          int `json:"count"`
		ScannedTickets int `json:"scanned_tickets"`
		Agents         []struct {
			AgentID     string  `json:"agentId"`
			Name        string  `json:"name"`
			OpenTickets int     `json:"openTickets"`
			Load        float64 `json:"load"`
			AboveMedian bool    `json:"aboveMedian"`
		} `json:"agents"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(view.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d: %s", len(view.Agents), out)
	}
	top := view.Agents[0]
	if top.AgentID != "A1" || top.OpenTickets != 3 || top.Name != "Ada Lovelace" {
		t.Fatalf("expected A1 'Ada Lovelace' with 3 open on top, got %+v", top)
	}
	if !top.AboveMedian {
		t.Errorf("expected most-loaded agent above median")
	}
}
