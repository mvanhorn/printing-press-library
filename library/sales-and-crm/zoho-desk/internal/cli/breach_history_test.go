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

// TestNovelBreachHistoryHelpWires smoke-tests that the breach-history command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBreachHistoryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"breach-history", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("breach-history --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "breach-history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("breach-history --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelBreachHistoryBehavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Upsert("agents", "A1", mustJSON(t, map[string]any{"id": "A1", "firstName": "Ada", "lastName": "L"}))
	now := time.Now()
	// Open and overdue -> breach for A1.
	db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "status": "Open", "assigneeId": "A1", "dueDate": zohoTime(now.Add(-5 * time.Hour)),
	}))
	// Future due -> not a breach, but counts toward A1 total.
	db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "status": "Open", "assigneeId": "A1", "dueDate": zohoTime(now.Add(50 * time.Hour)),
	}))
	// Closed on time -> not a breach.
	db.Upsert("tickets", "3", mustJSON(t, map[string]any{
		"id": "3", "status": "Closed", "assigneeId": "A1",
		"dueDate": zohoTime(now.Add(-10 * time.Hour)), "closedTime": zohoTime(now.Add(-12 * time.Hour)),
	}))
	db.Close()

	out := runTranscendCmd(t, "breach-history", "--by", "agent", "--db", dbPath, "--json")
	var view struct {
		By            string `json:"by"`
		TotalBreaches int    `json:"totalBreaches"`
		Groups        []struct {
			Key      string  `json:"key"`
			Name     string  `json:"name"`
			Breaches int     `json:"breaches"`
			Total    int     `json:"total"`
			Rate     float64 `json:"breachRate"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.By != "agent" || view.TotalBreaches != 1 {
		t.Errorf("by=%q totalBreaches=%d, want agent/1", view.By, view.TotalBreaches)
	}
	if len(view.Groups) != 1 {
		t.Fatalf("expected 1 group, got %+v", view.Groups)
	}
	g := view.Groups[0]
	if g.Key != "A1" || g.Name != "Ada L" || g.Breaches != 1 || g.Total != 3 {
		t.Fatalf("unexpected group: %+v", g)
	}
}
