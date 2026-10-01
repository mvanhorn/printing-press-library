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

// TestNovelLedgerSqlHelpWires smoke-tests that the ledger sql command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLedgerSqlHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"ledger", "sql", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ledger sql --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "sql"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ledger sql --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLedgerSqlCount(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "sql", "SELECT COUNT(*) AS n FROM accounts WHERE tombstone = 0")
	if len(rows) != 1 || rows[0]["n"] != float64(4) {
		t.Fatalf("rows = %v, want [{n:4}]", rows)
	}
}

func TestLedgerSqlLimitAndText(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "sql", "SELECT id, name FROM accounts WHERE tombstone = 0 ORDER BY sort_order;", "--limit", "2")
	if len(rows) != 2 || rows[0]["name"] != "Checking" || rows[1]["id"] != "acct-card" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestLedgerSqlRejectsWrites(t *testing.T) {
	s3Fixture(t)
	for _, q := range []string{"DELETE FROM accounts", "UPDATE accounts SET name = 'x'", "SELECT 1; DROP TABLE accounts", "PRAGMA writable_schema = 1"} {
		if _, _, err := s3Run(t, "ledger", "sql", q); s3ExitCode(err) != 2 {
			t.Errorf("%q: code %d err %v, want 2", q, s3ExitCode(err), err)
		}
	}
	// Mirror is untouched.
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "sql", "SELECT COUNT(*) AS n FROM accounts")
	if rows[0]["n"] != float64(5) {
		t.Fatalf("accounts changed: %v", rows)
	}
}

func TestLedgerSqlSchema(t *testing.T) {
	s3Fixture(t)
	var tables []map[string]any
	s3RunJSON(t, &tables, "ledger", "sql", "--schema")
	found := false
	for _, r := range tables {
		if r["name"] == "transactions" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--schema missing transactions: %v", tables)
	}
	var cols []map[string]any
	s3RunJSON(t, &cols, "ledger", "sql", "--schema", "accounts")
	if len(cols) == 0 || cols[0]["name"] != "id" {
		t.Fatalf("accounts columns = %v", cols)
	}
	if _, _, err := s3Run(t, "ledger", "sql", "--schema", "nope"); s3ExitCode(err) != 3 {
		t.Fatalf("unknown table: code %d, want 3", s3ExitCode(err))
	}
	var info []map[string]any
	s3RunJSON(t, &info, "ledger", "sql", "PRAGMA table_info(payees)")
	if len(info) == 0 {
		t.Fatal("PRAGMA table_info returned nothing")
	}
}

func TestLedgerSqlNoMirror(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "ledger", "sql", "SELECT 1")
	if err != nil || out != "[]\n" {
		t.Fatalf("out = %q err = %v, want []", out, err)
	}
}
