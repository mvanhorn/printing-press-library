// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelLedgerDuplicatesHelpWires smoke-tests that the ledger duplicates command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLedgerDuplicatesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"ledger", "duplicates", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ledger duplicates --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "duplicates"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ledger duplicates --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLedgerDuplicatesDefault(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "duplicates")
	if len(rows) != 1 || rows[0]["a_id"] != "txn-dup-a" || rows[0]["b_id"] != "txn-dup-b" {
		t.Fatalf("pairs = %v, want exactly txn-dup-a/txn-dup-b", rows)
	}
	r := rows[0]
	if r["amount"] != float64(-4210) || r["a_date"] != "2026-09-15" || r["b_date"] != "2026-09-16" || r["payee"] != "Chipotle" {
		t.Fatalf("pair fields = %v", r)
	}
	if c, _ := r["confidence"].(float64); c < 0.6 || c > 1 {
		t.Fatalf("confidence = %v", r["confidence"])
	}
	for _, k := range []string{"account", "reason"} {
		if s, _ := r[k].(string); s == "" {
			t.Fatalf("missing %s: %v", k, r)
		}
	}
}

func TestLedgerDuplicatesNegative(t *testing.T) {
	s3Fixture(t)
	for _, args := range [][]string{
		{"--days", "0"},
		{"--month", "2026-07"},
		{"--account", "Checking"},
		{"--min-confidence", "0.99"},
	} {
		out, _, err := s3Run(t, append([]string{"ledger", "duplicates"}, args...)...)
		if err != nil || out != "[]\n" {
			t.Errorf("%v: out = %q err = %v, want []", args, out, err)
		}
	}
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "duplicates", "--days", "30", "--min-confidence", "0")
	for _, r := range rows {
		if r["a_id"] == "txn-ch-2" || r["b_id"] == "txn-ch-2" {
			t.Fatalf("txn-ch-2 (different amount) paired: %v", r)
		}
	}
	if _, _, err := s3Run(t, "ledger", "duplicates", "--days", "-1"); s3ExitCode(err) != 2 {
		t.Fatalf("--days -1: code %d, want 2", s3ExitCode(err))
	}
}

func TestLedgerDuplicatesNoMirror(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "ledger", "duplicates")
	if err != nil || out != "[]\n" {
		t.Fatalf("out = %q err = %v, want []", out, err)
	}
}
