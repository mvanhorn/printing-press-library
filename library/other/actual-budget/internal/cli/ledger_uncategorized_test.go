// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestLedgerUncategorized(t *testing.T) {
	s3Fixture(t)
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "uncategorized")
	want := []string{"txn-am-2", "txn-ch-2", "txn-dup-a", "txn-dup-b", "txn-kr-6"}
	if got := s3IDs(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v (transfers, split parent, starting balances, off-budget must be excluded)", got, want)
	}
}

func TestLedgerUncategorizedFilters(t *testing.T) {
	s3Fixture(t)
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--month", "2026-08"}, []string{"txn-am-2"}},
		{[]string{"--account", "acct-checking"}, []string{}},
		{[]string{"--limit", "2"}, []string{"txn-dup-a", "txn-dup-b"}},
	}
	for _, tc := range cases {
		var rows []map[string]any
		s3RunJSON(t, &rows, append([]string{"ledger", "uncategorized"}, tc.args...)...)
		if got := s3IDs(rows); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v: ids = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestLedgerUncategorizedNoMirror(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "ledger", "uncategorized")
	if err != nil || out != "[]\n" {
		t.Fatalf("out = %q err = %v, want []", out, err)
	}
}
