// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestEnsureTable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := EnsureTable(ctx, db); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Idempotent.
	if err := EnsureTable(ctx, db); err != nil {
		t.Fatalf("second call (should be idempotent): %v", err)
	}
}

func TestSaveAndLatest(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	payload := json.RawMessage(`{"score": 14}`)
	if err := Save(ctx, db, "attention", now, payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Latest(ctx, db, "attention")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got == nil {
		t.Fatal("Latest returned nil for present snapshot")
	}
	if got.Metric != "attention" {
		t.Errorf("metric: got %q want attention", got.Metric)
	}
	if string(got.Data) != `{"score": 14}` {
		t.Errorf("data: got %q want %q", string(got.Data), `{"score": 14}`)
	}
}

func TestLatestEmpty(t *testing.T) {
	db := openTestDB(t)
	got, err := Latest(context.Background(), db, "missing")
	if err != nil {
		t.Fatalf("Latest on empty: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for absent snapshot, got %+v", got)
	}
}

func TestAtChoosesBefore(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	for i, ts := range []time.Time{t0, t1, t2} {
		data := json.RawMessage([]byte(`{"i":` + string(rune('0'+i)) + `}`))
		if err := Save(ctx, db, "attention", ts, data); err != nil {
			t.Fatalf("Save[%d]: %v", i, err)
		}
	}
	// Looking up at t1+1h should return t1, not t2.
	got, err := At(ctx, db, "attention", t1.Add(time.Hour))
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	if got == nil {
		t.Fatal("At returned nil")
	}
	if !got.TakenAt.Equal(t1) {
		t.Errorf("TakenAt: got %v want %v", got.TakenAt, t1)
	}
}

func TestListReturnsAllNewestFirst(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	for _, ts := range []time.Time{t0, t1} {
		_ = Save(ctx, db, "x", ts, json.RawMessage(`{}`))
	}
	list, err := List(ctx, db, "x", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len: got %d want 2", len(list))
	}
	if !list[0].TakenAt.Equal(t1) {
		t.Errorf("newest-first: got %v want %v", list[0].TakenAt, t1)
	}
}

func TestResolveAnchor(t *testing.T) {
	now := time.Date(2026, 5, 22, 8, 45, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"now", now, true},
		{"today", time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), true},
		{"yesterday", time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC), true},
		{"2h ago", now.Add(-2 * time.Hour), true},
		{"30m ago", now.Add(-30 * time.Minute), true},
		{"7d ago", now.AddDate(0, 0, -7), true},
		{"2026-05-15", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), true},
		{"garbage", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := ResolveAnchor(now, c.in)
		if ok != c.ok {
			t.Errorf("ResolveAnchor(%q): ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if c.ok && !got.Equal(c.want) {
			t.Errorf("ResolveAnchor(%q): got %v want %v", c.in, got, c.want)
		}
	}
}
