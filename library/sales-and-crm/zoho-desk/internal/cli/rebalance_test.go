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

// TestNovelRebalanceHelpWires smoke-tests that the rebalance command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRebalanceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rebalance", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rebalance --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "rebalance"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rebalance --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelRebalanceBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Upsert("agents", "A1", mustJSON(t, map[string]any{"id": "A1", "firstName": "Ada", "lastName": "L"}))
	db.Upsert("agents", "A2", mustJSON(t, map[string]any{"id": "A2", "firstName": "Bob", "lastName": "J"}))
	// A1: 3 open, A2: 1 open -> gap 2, one move expected to even them at 2/2.
	for _, id := range []string{"1", "2", "3"} {
		db.Upsert("tickets", id, mustJSON(t, map[string]any{"id": id, "status": "Open", "assigneeId": "A1"}))
	}
	db.Upsert("tickets", "4", mustJSON(t, map[string]any{"id": "4", "status": "Open", "assigneeId": "A2"}))
	db.Close()

	out := runTranscendCmd(t, "rebalance", "--plan", "--db", dbPath, "--json")
	var view struct {
		Mode         string `json:"mode"`
		MovesApplied int    `json:"movesApplied"`
		Moves        []struct {
			TicketID    string `json:"ticketId"`
			FromAgentID string `json:"fromAgentId"`
			ToAgentID   string `json:"toAgentId"`
		} `json:"moves"`
		ScannedTickets int `json:"scanned_tickets"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.Mode != "plan" || view.MovesApplied != 0 {
		t.Errorf("expected plan mode with 0 applied, got mode=%q applied=%d", view.Mode, view.MovesApplied)
	}
	if len(view.Moves) != 1 {
		t.Fatalf("expected exactly 1 move, got %+v", view.Moves)
	}
	if view.Moves[0].FromAgentID != "A1" || view.Moves[0].ToAgentID != "A2" {
		t.Errorf("expected move A1->A2, got %+v", view.Moves[0])
	}
}
