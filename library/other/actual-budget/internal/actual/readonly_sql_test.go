// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
)

// TestReadOnlySQLAdversarial runs evasion payloads through the same path
// ledger sql uses: CheckReadOnlySQL, the LIMIT wrapper, then a query on the
// read-only mirror connection. Whatever the guard accepts must fail or
// read only: the mirror bytes stay identical and no file appears.
func TestReadOnlySQLAdversarial(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	if err := actualtest.Build(dbPath); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.db")
	q := func(s string) string { return strings.ReplaceAll(s, "OUT", out) }
	payloads := []string{
		"AT/**/TACH 'OUT' AS z",
		"SELECT 1 /* nested /* */ ; ATTACH 'OUT' AS z; -- */",
		"SELECT 1 --",
		"SELECT [a']; ATTACH 'OUT' AS z; --]",
		"SELECT `a'`; VACUUM INTO 'OUT'",
		"SELECT 'it''s'; VACUUM INTO 'OUT'",
		"SELECT 'a\\'; VACUUM INTO 'OUT'; --'",
		"SELECT E'x'; VACUUM INTO 'OUT'",
		"SELECT 1) ; VACUUM INTO 'OUT'; SELECT (1",
		"SELECT 1 WHERE 0) UNION SELECT name FROM sqlite_master WHERE (1",
		"WITH x AS (SELECT 1) DELETE FROM transactions",
		"WITH x AS (SELECT 1) INSERT INTO accounts(id) VALUES ('evil')",
		"WITH x AS (SELECT 1) UPDATE accounts SET name = 'evil'",
		"SELECT * FROM pragma_query_only(0)",
		"SELECT * FROM pragma_writable_schema(1)",
		"SELECT * FROM pragma_journal_mode('delete')",
		"SELECT * FROM sqlite_dbpage",
		"SELECT fts3_tokenizer('simple')",
		"SELECT writefile('OUT', 'x')",
		"SELECT readfile('/etc/passwd')",
		"SELECT edit('x')",
		"SELECT * FROM zipfile('OUT')",
		"SELECT load_extension('x')",
		"SELECT 1; VACUUM INTO 'OUT'",
		"VACUUM\tINTO 'OUT'",
		"pRaGmA query_only = 0",
		"PRAGMA table_info(accounts) ; PRAGMA query_only=0",
		"PRAGMA table_info((SELECT 1)); ATTACH 'OUT' AS z",
		"SELECT 1 " + strings.Repeat("/* x */", 5000) + "; VACUUM INTO 'OUT'",
		"SELECT \"x;\"; VACUUM INTO 'OUT'",
		"SELECT 1 /*",
		"SELECT '",
		"SELECT [",
	}
	db, err := actual.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	l := actual.NewLedger(db)
	for _, p := range payloads {
		stmt, wrap, err := actual.CheckReadOnlySQL(q(p))
		if err != nil {
			continue
		}
		// Run it both as --limit 0 sends it and inside the LIMIT wrapper.
		runs := []string{stmt}
		if wrap {
			runs = append(runs, "SELECT * FROM ("+stmt+"\n) LIMIT 50")
		}
		for _, r := range runs {
			_, _, qerr := l.QueryRows(context.Background(), r)
			t.Logf("accepted by guard: %q -> query err: %v", p, qerr)
		}
		if _, statErr := os.Stat(out); statErr == nil {
			t.Fatalf("payload %q created %s", p, out)
		}
	}
	db.Close()
	after, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("mirror database bytes changed")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if n := e.Name(); n != "db.sqlite" && !strings.HasPrefix(n, "db.sqlite-") {
			t.Fatalf("unexpected file %s", n)
		}
	}
}
