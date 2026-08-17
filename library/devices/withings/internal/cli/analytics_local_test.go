// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared test helpers + statistics tests for the local analytics
// layer. Each command's own *_impl_test.go inserts synthetic rows via these
// helpers and asserts the computed output.

package cli

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
)

// newTestStore opens a fresh read-write store in a per-test temp dir and
// returns it with the resolved path.
func newTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "withings-test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

// upsertJSON marshals v and upserts it under (resourceType, id). Fails the test
// on any error so callers stay terse.
func upsertJSON(t *testing.T, s *store.Store, resourceType, id string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s/%s: %v", resourceType, id, err)
	}
	if err := s.Upsert(resourceType, id, raw); err != nil {
		t.Fatalf("upsert %s/%s: %v", resourceType, id, err)
	}
}

// daysAgoEpoch returns the unix epoch (seconds) for UTC midnight n days before
// now — handy for placing synthetic measure groups inside a lookback window.
func daysAgoEpoch(n int) int64 {
	return time.Now().UTC().AddDate(0, 0, -n).Unix()
}

// daysAgoYMD returns the YYYY-MM-DD key for n days before now (UTC).
func daysAgoYMD(n int) string {
	return time.Now().UTC().AddDate(0, 0, -n).Format("2006-01-02")
}

// measureGrp builds a measure-group JSON value with the given grpid, date, and
// (type,value,unit) measures.
func measureGrp(grpid int64, date int64, measures ...measureValue) map[string]any {
	ms := make([]map[string]any, 0, len(measures))
	for _, m := range measures {
		ms = append(ms, map[string]any{"value": m.Value, "type": m.Type, "unit": m.Unit})
	}
	return map[string]any{
		"grpid":    grpid,
		"category": 1,
		"date":     date,
		"measures": ms,
	}
}

func TestPearson(t *testing.T) {
	// Perfect positive correlation.
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{2, 4, 6, 8, 10}
	r, ok := pearson(xs, ys)
	if !ok {
		t.Fatal("pearson returned ok=false for a defined correlation")
	}
	if math.Abs(r-1.0) > 1e-9 {
		t.Errorf("pearson(perfect+) = %v, want 1.0", r)
	}

	// Perfect negative correlation.
	ys2 := []float64{10, 8, 6, 4, 2}
	r2, ok2 := pearson(xs, ys2)
	if !ok2 || math.Abs(r2+1.0) > 1e-9 {
		t.Errorf("pearson(perfect-) = %v (ok=%v), want -1.0", r2, ok2)
	}

	// Zero variance => undefined.
	if _, ok := pearson([]float64{3, 3, 3}, []float64{1, 2, 3}); ok {
		t.Error("pearson with zero-variance x should be undefined")
	}
	// Too few points.
	if _, ok := pearson([]float64{1}, []float64{1}); ok {
		t.Error("pearson with one point should be undefined")
	}
}

func TestRollingAvgTail(t *testing.T) {
	if got := rollingAvgTail([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 7); math.Abs(got-7.0) > 1e-9 {
		t.Errorf("rollingAvgTail last 7 of 1..10 = %v, want 7", got)
	}
	if got := rollingAvgTail([]float64{2, 4}, 7); math.Abs(got-3.0) > 1e-9 {
		t.Errorf("rollingAvgTail fewer than window = %v, want 3", got)
	}
	if got := rollingAvgTail(nil, 7); got != 0 {
		t.Errorf("rollingAvgTail empty = %v, want 0", got)
	}
}
