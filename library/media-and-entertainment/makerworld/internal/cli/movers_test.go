// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestComputeMovers(t *testing.T) {
	prev := map[string]snapshotMetrics{
		"1": {Title: "A", Download: 100},
		"2": {Title: "B", Download: 50},
		"3": {Title: "C", Download: 10},
	}
	cur := map[string]snapshotMetrics{
		"1": {Title: "A", Download: 300}, // +200
		"2": {Title: "B", Download: 60},  // +10
		"3": {Title: "C", Download: 5},   // -5  (excluded: no positive delta)
		"4": {Title: "D", Download: 99},  // new (excluded: not in prev)
	}
	got := computeMovers(cur, prev, "downloads", 10)
	if len(got) != 2 {
		t.Fatalf("got %d movers want 2: %+v", len(got), got)
	}
	if got[0].ID != "1" || got[0].Delta != 200 {
		t.Errorf("top mover = %+v want id=1 delta=200", got[0])
	}
	if got[1].ID != "2" || got[1].Delta != 10 {
		t.Errorf("second mover = %+v want id=2 delta=10", got[1])
	}
}

func TestComputeMoversLimit(t *testing.T) {
	prev := map[string]snapshotMetrics{"1": {Download: 1}, "2": {Download: 1}}
	cur := map[string]snapshotMetrics{"1": {Download: 100}, "2": {Download: 50}}
	if got := computeMovers(cur, prev, "downloads", 1); len(got) != 1 {
		t.Errorf("limit=1 returned %d movers", len(got))
	}
}
