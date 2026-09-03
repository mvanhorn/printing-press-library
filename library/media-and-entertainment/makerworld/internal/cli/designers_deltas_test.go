// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestAggregateDesignerDeltas(t *testing.T) {
	prev := map[string]snapshotMetrics{
		"d1": {CreatorID: "c1", CreatorName: "Alice", Like: 10, Download: 100},
		"d2": {CreatorID: "c2", CreatorName: "Bob", Like: 5, Download: 50},
	}
	cur := map[string]snapshotMetrics{
		"d1": {CreatorID: "c1", CreatorName: "Alice", Like: 20, Download: 160}, // +10 like, +60 dl
		"d2": {CreatorID: "c2", CreatorName: "Bob", Like: 5, Download: 50},     // unchanged -> excluded
		"d3": {CreatorID: "c1", CreatorName: "Alice", Like: 0, Download: 0},    // new design for Alice
	}
	got := aggregateDesignerDeltas(cur, prev, 10)
	if len(got) != 1 {
		t.Fatalf("got %d deltas want 1 (unchanged Bob excluded): %+v", len(got), got)
	}
	a := got[0]
	if a.CreatorID != "c1" || a.NewDesigns != 1 || a.LikeDelta != 10 || a.DownloadDelta != 60 {
		t.Errorf("alice delta = %+v", a)
	}
	if len(a.NewDesignIDs) != 1 || a.NewDesignIDs[0] != "d3" {
		t.Errorf("new design ids = %v want [d3]", a.NewDesignIDs)
	}
}

func TestAggregateDesignerDeltasEmpty(t *testing.T) {
	snap := map[string]snapshotMetrics{"d1": {CreatorID: "c1", Download: 10}}
	if got := aggregateDesignerDeltas(snap, snap, 10); len(got) != 0 {
		t.Errorf("identical snapshots should yield no deltas, got %d", len(got))
	}
}
