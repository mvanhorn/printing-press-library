// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"reflect"
	"testing"
)

// Test fixtures use gb-prefixed builders to avoid colliding with other helpers
// in package cli's test files.
func gbBal(ccy, amt string) Balance { return Balance{CurrencyCode: ccy, Amount: amt} }

func gbMember(id int, bals ...Balance) GroupMember {
	return GroupMember{ID: id, Balance: bals}
}

func gbGroup(id int, name string, members ...GroupMember) Group {
	return Group{ID: id, Name: name, Members: members}
}

// groupBalances returns the current user's net balance in each group they are a
// member of, one row per (group, currency) with a non-zero balance.

func TestGroupBalances_SingleNonZeroBalance(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Tahoe Trip", gbMember(42, gbBal("USD", "25.00")), gbMember(7, gbBal("USD", "-25.00"))),
	}
	got := groupBalances(groups, 42)
	want := []groupBalanceRow{
		{GroupID: 1, GroupName: "Tahoe Trip", CurrencyCode: "USD", Amount: 25.00},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupBalances = %+v, want %+v", got, want)
	}
}

func TestGroupBalances_ExcludesZeroBalanceGroups(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Settled Up", gbMember(42, gbBal("USD", "0.00"))),
		gbGroup(2, "Active", gbMember(42, gbBal("USD", "12.50"))),
	}
	got := groupBalances(groups, 42)
	want := []groupBalanceRow{
		{GroupID: 2, GroupName: "Active", CurrencyCode: "USD", Amount: 12.50},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupBalances = %+v, want %+v", got, want)
	}
}

func TestGroupBalances_MultiCurrencyEmitsRowPerCurrency(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Barcelona", gbMember(42, gbBal("USD", "10.00"), gbBal("EUR", "-5.00"))),
	}
	got := groupBalances(groups, 42)
	// Within the result the larger absolute amount comes first.
	want := []groupBalanceRow{
		{GroupID: 1, GroupName: "Barcelona", CurrencyCode: "USD", Amount: 10.00},
		{GroupID: 1, GroupName: "Barcelona", CurrencyCode: "EUR", Amount: -5.00},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupBalances = %+v, want %+v", got, want)
	}
}

func TestGroupBalances_SkipsGroupsWhereUserIsNotMember(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Not Mine", gbMember(7, gbBal("USD", "99.00"))),
		gbGroup(2, "Mine", gbMember(42, gbBal("USD", "3.00"))),
	}
	got := groupBalances(groups, 42)
	want := []groupBalanceRow{
		{GroupID: 2, GroupName: "Mine", CurrencyCode: "USD", Amount: 3.00},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupBalances = %+v, want %+v", got, want)
	}
}

func TestGroupBalances_SortedByAbsoluteAmountDescending(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Small", gbMember(42, gbBal("USD", "5.00"))),
		gbGroup(2, "Big", gbMember(42, gbBal("USD", "-40.00"))),
		gbGroup(3, "Mid", gbMember(42, gbBal("USD", "12.00"))),
	}
	got := groupBalances(groups, 42)
	wantOrder := []string{"Big", "Mid", "Small"}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(wantOrder), got)
	}
	for i, name := range wantOrder {
		if got[i].GroupName != name {
			t.Fatalf("row %d = %q, want %q (full: %+v)", i, got[i].GroupName, name, got)
		}
	}
}

func TestGroupBalances_TieBreaksByGroupName(t *testing.T) {
	groups := []Group{
		gbGroup(1, "Zebra", gbMember(42, gbBal("USD", "20.00"))),
		gbGroup(2, "Alpha", gbMember(42, gbBal("USD", "-20.00"))),
	}
	got := groupBalances(groups, 42)
	wantOrder := []string{"Alpha", "Zebra"}
	for i, name := range wantOrder {
		if got[i].GroupName != name {
			t.Fatalf("row %d = %q, want %q (full: %+v)", i, got[i].GroupName, name, got)
		}
	}
}

func TestGroupBalances_EmptyGroupsReturnsEmpty(t *testing.T) {
	got := groupBalances(nil, 42)
	if len(got) != 0 {
		t.Fatalf("groupBalances(nil) = %+v, want empty", got)
	}
}
