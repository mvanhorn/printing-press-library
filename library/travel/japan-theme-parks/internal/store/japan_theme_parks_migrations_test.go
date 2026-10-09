// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitSnapshotsDedupe(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "h.db")
	s, err := OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	src := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	rows := []WaitRow{
		{ParkID: 275, RideID: 1, RideName: "A", IsOpen: true, WaitMinutes: 30, SourceUpdatedAt: src, FetchedAt: src},
		{ParkID: 275, RideID: 2, RideName: "B", IsOpen: false, SourceUpdatedAt: src, FetchedAt: src},
		{ParkID: 275, RideID: 3, RideName: "no timestamp", IsOpen: true, WaitMinutes: 5, FetchedAt: src},
	}
	c, err := s.InsertWaitRows(ctx, rows)
	if err != nil || c[275].Inserted != 2 || c[275].Skipped != 0 || c[275].Invalid != 1 {
		t.Fatalf("first insert = %+v, %v", c, err)
	}
	c, err = s.InsertWaitRows(ctx, rows)
	if err != nil || c[275].Inserted != 0 || c[275].Skipped != 2 {
		t.Fatalf("second insert = %+v, %v", c, err)
	}
	_ = s.Close()

	ro, err := OpenReadOnlyContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	got, bad, err := ro.QueryWaitRows(ctx, WaitQuery{ParkID: 275})
	if err != nil || bad != 0 || len(got) != 2 || !got[0].SourceUpdatedAt.Equal(src) {
		t.Fatalf("load = %+v, %v", got, err)
	}
	// src is 2026-10-09 01:00 UTC = Friday 10:00 in Tokyo.
	cases := []struct {
		name string
		q    WaitQuery
		want int
	}{
		{"weekday match", WaitQuery{ParkID: 275, Weekdays: []int{5}}, 2},
		{"weekday miss", WaitQuery{ParkID: 275, Weekdays: []int{6, 0}}, 0},
		{"weekday set with match", WaitQuery{ParkID: 275, Weekdays: []int{0, 5, 6}}, 2},
		{"weekday 15 is not 5", WaitQuery{ParkID: 275, Weekdays: []int{15}}, 0},
		{"weekday and hour and ride", WaitQuery{ParkID: 275, Weekdays: []int{5}, HourSet: true, HourFrom: 9, HourTo: 11, RideLike: "a"}, 1},
		{"hour match", WaitQuery{ParkID: 275, HourSet: true, HourFrom: 10, HourTo: 10}, 2},
		{"hour miss", WaitQuery{ParkID: 275, HourSet: true, HourFrom: 1, HourTo: 9}, 0},
		{"ride like", WaitQuery{ParkID: 275, RideLike: "a"}, 1},
		{"like escapes percent", WaitQuery{ParkID: 275, RideLike: "%"}, 0},
	}
	for _, tc := range cases {
		rows, _, err := ro.QueryWaitRows(ctx, tc.q)
		if err != nil || len(rows) != tc.want {
			t.Errorf("%s: got %d rows, %v", tc.name, len(rows), err)
		}
	}
	span, err := ro.WaitHistorySpan(ctx, 275)
	if err != nil || span.Rows != 2 || !span.First.Equal(src) || !span.Last.Equal(src) {
		t.Fatalf("span = %+v, %v", span, err)
	}
	none, _, err := ro.QueryWaitRows(ctx, WaitQuery{ParkID: 999})
	if err != nil || len(none) != 0 {
		t.Fatalf("other park = %+v, %v", none, err)
	}
}

func TestWaitSnapshotsCreatedOnOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "empty.db")
	s, err := OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'wait_snapshots'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("wait_snapshots not created by migrateExtras: n=%d err=%v", n, err)
	}
	got, _, err := s.QueryWaitRows(ctx, WaitQuery{ParkID: 275})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestWaitHistoryByPark(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "h.db")
	s, err := OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	if _, err := s.InsertWaitRows(ctx, []WaitRow{
		{ParkID: 275, RideID: 1, RideName: "A", SourceUpdatedAt: t1, FetchedAt: t1},
		{ParkID: 275, RideID: 1, RideName: "A", SourceUpdatedAt: t2, FetchedAt: t2},
		{ParkID: 274, RideID: 9, RideName: "B", SourceUpdatedAt: t1, FetchedAt: t1},
	}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	ro, err := OpenReadOnlyContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	got, err := ro.WaitHistoryByPark(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].ParkID != 274 || got[0].Rows != 1 || got[1].ParkID != 275 || got[1].Rows != 2 ||
		!got[1].First.Equal(t1) || !got[1].Last.Equal(t2) {
		t.Fatalf("summary = %+v", got)
	}
	if err := ro.RejectNewerSchema(); err != nil {
		t.Fatalf("current schema rejected: %v", err)
	}
}

// A read-only open does not run the writable open's version gate, so history
// readers must reject a database stamped by a newer binary themselves.
func TestRejectNewerSchemaOnReadOnlyOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "future.db")
	s, err := OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", StoreSchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	ro, err := OpenReadOnlyContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	err = ro.RejectNewerSchema()
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("RejectNewerSchema = %v, want newer-schema error", err)
	}
}
