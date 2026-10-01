// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelChangesSinceHelpWires smoke-tests that the changes since command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelChangesSinceHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"changes", "since", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("changes since --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "since"} {
		if !strings.Contains(help, want) {
			t.Fatalf("changes since --help missing %q in output:\n%s", want, help)
		}
	}
}

type s3ChangeRow struct {
	Dataset string         `json:"dataset"`
	Row     string         `json:"row"`
	Action  string         `json:"action"`
	Entity  string         `json:"entity"`
	Changes map[string]any `json:"changes"`
	Device  string         `json:"device"`
}

func s3ChangesByRow(rows []s3ChangeRow) map[string]s3ChangeRow {
	by := map[string]s3ChangeRow{}
	for _, r := range rows {
		by[r.Dataset+"/"+r.Row] = r
	}
	return by
}

func TestChangesSince30Days(t *testing.T) {
	s3Fixture(t)
	var rows []s3ChangeRow
	s3RunJSON(t, &rows, "changes", "since", "30d", "--as-of", "2026-09-30")
	by := s3ChangesByRow(rows)
	del := by["transactions/txn-deleted"]
	if del.Action != "deleted" || !strings.Contains(del.Entity, "Kroger") || del.Device != "bbbbbbbbbbbbbbbb" {
		t.Errorf("txn-deleted: %+v", del)
	}
	rent := by["transactions/txn-rent-09"]
	if rent.Action != "edited" || rent.Changes["amount"] != float64(-165000) {
		t.Errorf("txn-rent-09: %+v", rent)
	}
	if car := by["categories/cat-car"]; car.Action != "edited" || car.Changes["name"] != "Car Fund" || car.Entity != "Car Fund" {
		t.Errorf("cat-car: %+v", car)
	}
	if _, ok := by["payees/pay-chipotle"]; ok {
		t.Error("2026-08-01 payees message must be outside a 30-day window")
	}
}

func TestChangesSinceAbsoluteDateAndDataset(t *testing.T) {
	s3Fixture(t)
	var rows []s3ChangeRow
	s3RunJSON(t, &rows, "changes", "since", "2026-09-17")
	by := s3ChangesByRow(rows)
	if _, ok := by["transactions/txn-dup-a"]; ok {
		t.Error("09-15 change included")
	}
	if _, ok := by["transactions/txn-rent-09"]; ok {
		t.Error("09-16 change included")
	}
	if _, ok := by["transactions/txn-deleted"]; !ok {
		t.Error("09-17 change missing")
	}
	rows = nil
	s3RunJSON(t, &rows, "changes", "since", "2026-09-01", "--dataset", "categories")
	if len(rows) != 1 || rows[0].Row != "cat-car" {
		t.Errorf("--dataset categories = %+v", rows)
	}
}

func TestChangesSinceValidationAndEmpty(t *testing.T) {
	s3Fixture(t)
	if _, _, err := s3Run(t, "changes", "since", "yesterday-ish"); s3ExitCode(err) != 2 {
		t.Errorf("bad when: exit %d", s3ExitCode(err))
	}
	s3NoMirror(t)
	out, _, err := s3Run(t, "changes", "since", "7d")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v", out, err)
	}
}

func TestChangesSinceResolvesNames(t *testing.T) {
	s3Fixture(t)
	var rows []actual.RowChange
	s3RunJSON(t, &rows, "changes", "since", "2026-01-01", "--dataset", "transactions", "--limit", "0")
	resolved := 0
	for _, r := range rows {
		if r.Row == "txn-dup-a" {
			if r.Changes["description"] != "Chipotle" || r.Changes["description_id"] != "pay-chipotle" {
				t.Errorf("txn-dup-a payee change = %v / %v, want Chipotle / pay-chipotle", r.Changes["description"], r.Changes["description_id"])
			}
			resolved++
		}
		if _, ok := r.Changes["raw_synced_data"]; ok {
			t.Errorf("%s: raw_synced_data not dropped", r.Row)
		}
		if id, ok := r.Changes["description_id"].(string); ok {
			if name, _ := r.Changes["description"].(string); name == "" || name == id {
				t.Errorf("%s: payee id %s not resolved to a name (%v)", r.Row, id, r.Changes["description"])
			}
		}
		if d, ok := r.Changes["date"]; ok {
			if _, isStr := d.(string); !isStr {
				t.Errorf("%s: date %v not ISO", r.Row, d)
			}
		}
	}
	if resolved != 1 {
		t.Fatalf("txn-dup-a not in changes: %d rows", len(rows))
	}
}
