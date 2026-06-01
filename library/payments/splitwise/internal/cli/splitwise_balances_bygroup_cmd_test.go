// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
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

// TestBalancesByGroupCommand drives the balances command end-to-end with
// --by-group set against a seeded offline store, locking in the JSON output
// contract (the "by_group" envelope key, computed from synced group members)
// and the precedence of --by-group over --by-currency.
func TestBalancesByGroupCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbPath := defaultDBPath("splitwise-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Current user (id 42) is a member of "Tahoe" with a non-zero USD balance.
	if err := s.Upsert("get-current-user", "42", []byte(`{"user":{"id":42}}`)); err != nil {
		t.Fatalf("seed current user: %v", err)
	}
	if err := s.Upsert("get-groups", "100", []byte(`{"id":100,"name":"Tahoe","members":[{"id":42,"balance":[{"currency_code":"USD","amount":"25.00"}]},{"id":7,"balance":[{"currency_code":"USD","amount":"-25.00"}]}]}`)); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := &rootFlags{agent: true} // force the structured (JSON) output path
	cmd := newBalancesCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	// Both flags set: --by-group must win.
	cmd.SetArgs([]string{"--by-group", "--by-currency"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (stderr: %s)", err, errBuf.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON object: %v (raw: %s)", err, out.String())
	}
	rows, ok := got["by_group"].([]any)
	if !ok {
		t.Fatalf("expected by_group array in output, got: %s", out.String())
	}
	// --by-group precedence: the per-friend / per-currency envelopes must be absent.
	if _, present := got["friends"]; present {
		t.Errorf("--by-group should not emit a friends breakdown; got: %s", out.String())
	}
	if _, present := got["by_currency"]; present {
		t.Errorf("--by-group should take precedence over --by-currency; got: %s", out.String())
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 group row for the current user, got %d: %s", len(rows), out.String())
	}
	row := rows[0].(map[string]any)
	if row["group_name"] != "Tahoe" {
		t.Errorf("group_name = %v, want Tahoe", row["group_name"])
	}
	if row["amount"] != 25.0 {
		t.Errorf("amount = %v, want 25", row["amount"])
	}
}

// TestBalancesByGroupUnsyncedUserNoteInAgentMode guards the reachability of the
// "current user not synced" stderr note: it must fire in the structured/agent
// output path, not only the human-table path. Without a synced current user,
// groupBalances yields no rows; an agent reading {"by_group":[]} needs the note
// to distinguish "identity unknown" from "no balances".
func TestBalancesByGroupUnsyncedUserNoteInAgentMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbPath := defaultDBPath("splitwise-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Groups are synced, but get-current-user is NOT — so youID resolves to 0.
	if err := s.Upsert("get-groups", "100", []byte(`{"id":100,"name":"Tahoe","members":[{"id":42,"balance":[{"currency_code":"USD","amount":"25.00"}]}]}`)); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := &rootFlags{agent: true} // structured output path
	cmd := newBalancesCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--by-group"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (stderr: %s)", err, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "current user not synced") {
		t.Errorf("expected unsynced-current-user note on stderr in agent mode, got: %q", errBuf.String())
	}
}
