// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestLedgerSearchKroger(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "search", "kroger")
	want := []string{"txn-kr-1", "txn-kr-2", "txn-kr-3", "txn-kr-4", "txn-kr-5", "txn-kr-6"}
	if got := s3IDs(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for _, r := range rows {
		if r["id"] == "txn-deleted" {
			t.Fatal("tombstoned txn-deleted returned")
		}
		for _, k := range []string{"id", "date", "amount", "account", "payee", "category", "notes", "imported_payee"} {
			if _, ok := r[k]; !ok {
				t.Fatalf("row missing %q: %v", k, r)
			}
		}
	}
}

func TestLedgerSearchNotesOnly(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "search", "burrito")
	if got := s3IDs(rows); !reflect.DeepEqual(got, []string{"txn-ch-1"}) {
		t.Fatalf("ids = %v, want [txn-ch-1]", got)
	}
}

func TestLedgerSearchFilters(t *testing.T) {
	s3Fixture(t)
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"kroger", "--from", "2026-08-01", "--to", "2026-08-31"}, []string{"txn-kr-4", "txn-kr-5"}},
		{[]string{"kroger", "--account", "Checking"}, []string{}},
		{[]string{"kroger", "deli"}, []string{"txn-kr-5"}},
		{[]string{"kroger", "--limit", "1"}, []string{"txn-kr-6"}},
	}
	for _, tc := range cases {
		var rows []map[string]any
		s3RunJSON(t, &rows, append([]string{"ledger", "search"}, tc.args...)...)
		if got := s3IDs(rows); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v: ids = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestLedgerSearchNoMatch(t *testing.T) {
	s3Fixture(t)
	out, _, err := s3Run(t, "ledger", "search", "zzznomatch")
	if err != nil || out != "[]\n" {
		t.Fatalf("out = %q err = %v, want []", out, err)
	}
}

func TestLedgerSearchErrors(t *testing.T) {
	s3Fixture(t)
	if _, _, err := s3Run(t, "ledger", "search", "kroger", "--account", "Nope Bank"); s3ExitCode(err) != 3 {
		t.Fatalf("unknown account: code %d err %v, want 3", s3ExitCode(err), err)
	}
	if _, _, err := s3Run(t, "ledger", "search", "kroger", "--from", "bad"); s3ExitCode(err) != 2 {
		t.Fatalf("bad date: code %d, want 2", s3ExitCode(err))
	}
	if _, _, err := s3Run(t, "ledger", "search"); s3ExitCode(err) != 2 {
		t.Fatalf("missing query: code %d, want 2", s3ExitCode(err))
	}
}

func TestLedgerSearchNoMirror(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "ledger", "search", "kroger")
	if err != nil || out != "[]\n" {
		t.Fatalf("out = %q err = %v, want []", out, err)
	}
}
