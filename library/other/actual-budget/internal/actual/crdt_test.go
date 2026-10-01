// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestDecodeCRDTValue(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"S:Car Fund", "Car Fund"},
		{"S:", ""},
		{"N:-165000", int64(-165000)},
		{"N:1.5", 1.5},
		{"0:", nil},
		{"weird", "weird"},
	}
	for _, c := range cases {
		if got := actual.DecodeCRDTValue(c.in); got != c.want {
			t.Errorf("DecodeCRDTValue(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestParseCRDTTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		when string
		node string
		err  bool
	}{
		{"2026-09-15T10:00:00.000Z-0000-aaaaaaaaaaaaaaaa", "2026-09-15T10:00:00Z", "aaaaaaaaaaaaaaaa", false},
		{"2026-09-18T08:00:01.000Z-0012-bbbbbbbbbbbbbbbb", "2026-09-18T08:00:01Z", "bbbbbbbbbbbbbbbb", false},
		{"garbage", "", "", true},
	}
	for _, c := range cases {
		tm, node, err := actual.ParseCRDTTimestamp(c.in)
		if (err != nil) != c.err {
			t.Fatalf("%q err = %v", c.in, err)
		}
		if c.err {
			continue
		}
		if tm.UTC().Format(time.RFC3339) != c.when || node != c.node {
			t.Errorf("%q -> %s %s", c.in, tm.Format(time.RFC3339), node)
		}
	}
}

func TestGroupChanges(t *testing.T) {
	msgs := []actual.CRDTMessage{
		{Timestamp: "1", Node: "a", Dataset: "transactions", Row: "t1", Column: "amount", Value: int64(5)},
		{Timestamp: "2", Node: "b", Dataset: "transactions", Row: "t1", Column: "amount", Value: int64(7)},
		{Timestamp: "3", Node: "a", Dataset: "transactions", Row: "t2", Column: "tombstone", Value: int64(1)},
		{Timestamp: "4", Node: "a", Dataset: "payees", Row: "p1", Column: "tombstone", Value: int64(0)},
	}
	got := actual.GroupChanges(msgs)
	cases := []struct {
		i           int
		row, action string
		messages    int
		devices     int
	}{
		{0, "p1", "restored", 1, 1},
		{1, "t2", "deleted", 1, 1},
		{2, "t1", "edited", 2, 2},
	}
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	for _, c := range cases {
		g := got[c.i]
		if g.Row != c.row || g.Action != c.action || g.MessageCount != c.messages || len(g.Devices) != c.devices {
			t.Errorf("row %d = %+v", c.i, g)
		}
	}
	if got[2].Changes["amount"] != int64(7) || got[2].Device != "b" {
		t.Errorf("latest value/device not kept: %+v", got[2])
	}
	if len(actual.GroupChanges(nil)) != 0 {
		t.Error("empty input should give empty output")
	}
}

func TestCRDTMessagesAndDescribe(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	day := func(s string) time.Time { tm, _ := time.Parse("2006-01-02", s); return tm }
	cases := []struct {
		since, until time.Time
		dataset      string
		want         int
	}{
		{day("2026-08-31"), time.Time{}, "", 6},
		{day("2026-07-01"), time.Time{}, "", 7},
		{day("2026-09-17"), time.Time{}, "", 3},
		{day("2026-09-01"), day("2026-09-16"), "", 2},
		{day("2026-09-01"), time.Time{}, "transactions", 4},
	}
	for _, c := range cases {
		got, err := l.CRDTMessages(ctx, c.since, c.until, c.dataset)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != c.want {
			t.Errorf("since %s until %s %s: %d messages, want %d", c.since, c.until, c.dataset, len(got), c.want)
		}
	}
	descs := []struct{ ds, row, want string }{
		{"transactions", "txn-deleted", "Kroger -999.99 2026-09-01"},
		{"categories", "cat-car", "Car Fund"},
		{"payees", "pay-krogerold", "KROGER #123"},
		{"accounts", "acct-checking", "Checking"},
		{"zero_budgets", "202609-cat-car", "Car Fund 2026-09"},
		{"schedules", "sched-rent", "Rent"},
		{"transactions", "nope", ""},
		{"unknown_table", "x", ""},
	}
	for _, d := range descs {
		if got := l.DescribeEntity(ctx, d.ds, d.row); got != d.want {
			t.Errorf("DescribeEntity(%s,%s) = %q, want %q", d.ds, d.row, got, d.want)
		}
	}
}
