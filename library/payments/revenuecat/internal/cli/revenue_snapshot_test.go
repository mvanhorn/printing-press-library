// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotPersistAndDiff(t *testing.T) {
	db := newNovelTestStore(t)
	ctx := context.Background()

	// No prior snapshot for a fresh project.
	if _, _, hasPrior := loadPriorSnapshot(ctx, db, "proj1"); hasPrior {
		t.Fatal("expected no prior snapshot for fresh project")
	}

	// Persist a first snapshot.
	first := revenueSnapshotView{
		ProjectID:  "proj1",
		CapturedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		MRR:        1000,
		ARR:        12000,
		ActiveSubs: 200,
		Metrics: []snapshotMetric{
			{ID: "mrr", Value: 1000},
			{ID: "active_subscriptions", Value: 200},
		},
	}
	if err := persistSnapshot(ctx, db, first); err != nil {
		t.Fatalf("persist first: %v", err)
	}

	// Now a prior should exist with the right per-metric values.
	prior, priorAt, hasPrior := loadPriorSnapshot(ctx, db, "proj1")
	if !hasPrior {
		t.Fatal("expected prior snapshot after persist")
	}
	if priorAt != first.CapturedAt {
		t.Fatalf("priorAt = %q, want %q", priorAt, first.CapturedAt)
	}
	if prior["mrr"] != 1000 || prior["active_subscriptions"] != 200 {
		t.Fatalf("prior metrics = %+v", prior)
	}

	// A different project must not see proj1's snapshot.
	if _, _, has := loadPriorSnapshot(ctx, db, "proj2"); has {
		t.Fatal("project isolation broken: proj2 saw proj1 snapshot")
	}

	// Persist a newer snapshot and confirm it becomes the prior (most recent).
	second := revenueSnapshotView{
		ProjectID:  "proj1",
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		MRR:        1500,
		Metrics:    []snapshotMetric{{ID: "mrr", Value: 1500}},
	}
	if err := persistSnapshot(ctx, db, second); err != nil {
		t.Fatalf("persist second: %v", err)
	}
	prior2, _, _ := loadPriorSnapshot(ctx, db, "proj1")
	if prior2["mrr"] != 1500 {
		t.Fatalf("latest prior mrr = %v, want 1500", prior2["mrr"])
	}
}
