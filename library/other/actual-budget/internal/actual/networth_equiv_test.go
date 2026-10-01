// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

// netWorthPerMonth is the original NetWorth: one full Accounts aggregate per
// month end. NetWorth must produce exactly the same points.
func netWorthPerMonth(t *testing.T, l *actual.Ledger, months []int, byAccount bool) []actual.NetWorthPoint {
	t.Helper()
	out := make([]actual.NetWorthPoint, 0, len(months))
	var prev int64
	for i, ym := range months {
		end := actual.MonthEnd(ym)
		accts, err := l.Accounts(context.Background(), end)
		if err != nil {
			t.Fatal(err)
		}
		p := actual.NetWorthPoint{Month: actual.FormatYM(ym), AsOf: actual.FormatDate(end)}
		if byAccount {
			p.Accounts = make([]actual.AccountBalance, 0, len(accts))
		}
		for _, a := range accts {
			p.NetWorth += a.Balance
			if byAccount {
				p.Accounts = append(p.Accounts, actual.AccountBalance{ID: a.ID, Name: a.Name, OffBudget: a.OffBudget, Balance: a.Balance})
			}
		}
		if i > 0 {
			p.Change = p.NetWorth - prev
		}
		prev = p.NetWorth
		out = append(out, p)
	}
	return out
}

func TestNetWorthMatchesPerMonthAccounts(t *testing.T) {
	l := fixtureLedger(t)
	var months []int
	for ym := 202601; ym <= 202612; ym++ {
		months = append(months, ym)
	}
	cases := [][]int{
		months,
		{202605, 202606, 202607},
		{202609},
		{202509, 202701},
		{202608, 202606, 202609}, // out of order still matches
		{},
	}
	for _, ms := range cases {
		for _, byAccount := range []bool{false, true} {
			got, err := l.NetWorth(context.Background(), ms, byAccount)
			if err != nil {
				t.Fatal(err)
			}
			want := netWorthPerMonth(t, l, ms, byAccount)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("NetWorth(%v, %v) =\n%+v\nwant\n%+v", ms, byAccount, got, want)
			}
		}
	}
	// The fixture must actually move between months for this to mean much.
	pts := netWorthPerMonth(t, l, months, false)
	changed := 0
	for _, p := range pts {
		if p.Change != 0 {
			changed++
		}
	}
	if changed < 2 {
		t.Fatalf("fixture net worth changed in only %d months", changed)
	}
}
