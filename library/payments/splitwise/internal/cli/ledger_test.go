// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

// TestNovelLedgerHelpWires smoke-tests that the ledger command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLedgerHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"ledger", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ledger --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "ledger"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ledger --help missing %q in output:\n%s", want, help)
		}
	}
}

func seedLedgerStore(t *testing.T, friends ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	rows := map[string]map[string]string{
		"get-current-user": {"42": `{"user":{"id":42}}`},
		"get-groups": {
			"10": `{"id":10,"name":"Trip One"}`,
			"20": `{"id":20,"name":"Trip Two"}`,
		},
		"get-expenses": {
			"1": `{"id":1,"group_id":10,"date":"2026-01-01","description":"Dinner","currency_code":"USD","users":[{"user_id":42,"paid_share":"20","owed_share":"10"},{"user_id":7,"paid_share":"0","owed_share":"10"}]}`,
			"2": `{"id":2,"date":"2026-01-02","description":"Coffee","currency_code":"USD","users":[{"user_id":42,"paid_share":"0","owed_share":"3"},{"user_id":7,"paid_share":"6","owed_share":"3"}]}`,
			"3": `{"id":3,"group_id":20,"date":"2026-01-03","description":"Settlement","currency_code":"USD","payment":true,"users":[{"user_id":42,"paid_share":"0","owed_share":"4"},{"user_id":7,"paid_share":"4","owed_share":"0"}]}`,
		},
	}
	if len(friends) == 0 {
		friends = []string{`{"id":7,"first_name":"Alex","last_name":"Kim"}`}
	}
	rows["get-friends"] = make(map[string]string)
	for i, body := range friends {
		rows["get-friends"][string(rune('1'+i))] = body
	}
	for resource, entries := range rows {
		for id, body := range entries {
			if err := s.Upsert(resource, id, []byte(body)); err != nil {
				t.Fatalf("seed %s: %v", resource, err)
			}
		}
	}
	return path
}

func TestLedgerFriendReplaysAllGroupsInDateOrder(t *testing.T) {
	path := seedLedgerStore(t)
	flags := &rootFlags{agent: true}
	cmd := newNovelLedgerCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--friend", "Alex Kim", "--db", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (%s)", err, out.String())
	}
	var rows []friendLedgerRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %s", len(rows), out.String())
	}
	for i, want := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
		if rows[i].Date != want {
			t.Errorf("row %d date = %q, want %q", i, rows[i].Date, want)
		}
	}
	if rows[1].Group != "no group" {
		t.Errorf("non-group label = %q", rows[1].Group)
	}
	if rows[2].RunningBalance != 3 {
		t.Errorf("final running balance = %.2f, want 3.00", rows[2].RunningBalance)
	}
	if !rows[2].Payment {
		t.Error("settlement row payment = false")
	}
}

func TestLedgerFriendAmbiguousNameErrors(t *testing.T) {
	path := seedLedgerStore(t,
		`{"id":7,"first_name":"Alex","last_name":"A"}`,
		`{"id":8,"first_name":"Alex","last_name":"B"}`,
	)
	cmd := newNovelLedgerCmd(&rootFlags{agent: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--friend", "Alex", "--db", path})
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(out.String(), "ambiguous") {
		t.Fatalf("error = %v output=%q, want ambiguous envelope", err, out.String())
	}
}

func TestLedgerFriendAndGroupAreMutuallyExclusive(t *testing.T) {
	cmd := newNovelLedgerCmd(&rootFlags{})
	cmd.SetArgs([]string{"Trip", "--friend", "Alex"})
	err := cmd.Execute()
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", got, err)
	}
}

func TestLedgerFriendDryRunBeforeDatabaseAccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newNovelLedgerCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--friend", "X", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run accessed database: stat err=%v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("missing dry-run envelope: %s", out.String())
	}
}
