// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
package snapshots

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "snap.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCaptureLatestBeforePrior(t *testing.T) {
	db := openTemp(t)
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	// Three captures across time.
	if err := db.Capture("https://x.com", "queries", json.RawMessage(`[{"Query":"a"}]`), base); err != nil {
		t.Fatalf("capture1: %v", err)
	}
	if err := db.Capture("https://x.com", "queries", json.RawMessage(`[{"Query":"b"}]`), base.AddDate(0, 0, 5)); err != nil {
		t.Fatalf("capture2: %v", err)
	}
	if err := db.Capture("https://x.com", "queries", json.RawMessage(`[{"Query":"c"}]`), base.AddDate(0, 0, 10)); err != nil {
		t.Fatalf("capture3: %v", err)
	}

	latest, ok, err := db.Latest("https://x.com", "queries")
	if err != nil || !ok {
		t.Fatalf("Latest ok=%v err=%v", ok, err)
	}
	if string(latest.Data) != `[{"Query":"c"}]` {
		t.Errorf("Latest data = %s, want c", latest.Data)
	}

	// Before day 7 -> the day-5 capture (at-or-before).
	bef, ok, err := db.Before("https://x.com", "queries", base.AddDate(0, 0, 7))
	if err != nil || !ok {
		t.Fatalf("Before ok=%v err=%v", ok, err)
	}
	if string(bef.Data) != `[{"Query":"b"}]` {
		t.Errorf("Before(day7) = %s, want b", bef.Data)
	}

	// Prior to the latest capture -> the day-5 one (strictly before).
	pr, ok, err := db.Prior("https://x.com", "queries", base.AddDate(0, 0, 10))
	if err != nil || !ok {
		t.Fatalf("Prior ok=%v err=%v", ok, err)
	}
	if string(pr.Data) != `[{"Query":"b"}]` {
		t.Errorf("Prior(day10) = %s, want b", pr.Data)
	}

	all, err := db.All("https://x.com", "queries")
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("All len = %d, want 3", len(all))
	}
}

func TestEmptyStore(t *testing.T) {
	db := openTemp(t)
	if _, ok, err := db.Latest("https://x.com", "queries"); ok || err != nil {
		t.Errorf("Latest on empty: ok=%v err=%v, want false/nil", ok, err)
	}
	if _, ok, err := db.Before("https://x.com", "queries", time.Now()); ok || err != nil {
		t.Errorf("Before on empty: ok=%v err=%v, want false/nil", ok, err)
	}
	all, err := db.All("https://x.com", "queries")
	if err != nil || len(all) != 0 {
		t.Errorf("All on empty: len=%d err=%v", len(all), err)
	}
}
