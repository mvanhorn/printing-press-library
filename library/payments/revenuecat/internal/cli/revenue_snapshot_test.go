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

	// No prior snapshot for a fresh project: ok=false, err=nil (not an error).
	if _, _, hasPrior, err := loadPriorSnapshot(ctx, db, "proj1"); hasPrior || err != nil {
		t.Fatalf("fresh project: hasPrior=%v err=%v, want false/nil", hasPrior, err)
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
	prior, priorAt, hasPrior, err := loadPriorSnapshot(ctx, db, "proj1")
	if !hasPrior || err != nil {
		t.Fatalf("after persist: hasPrior=%v err=%v, want true/nil", hasPrior, err)
	}
	if priorAt != first.CapturedAt {
		t.Fatalf("priorAt = %q, want %q", priorAt, first.CapturedAt)
	}
	if prior["mrr"] != 1000 || prior["active_subscriptions"] != 200 {
		t.Fatalf("prior metrics = %+v", prior)
	}

	// A different project must not see proj1's snapshot.
	if _, _, has, _ := loadPriorSnapshot(ctx, db, "proj2"); has {
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
	prior2, _, _, _ := loadPriorSnapshot(ctx, db, "proj1")
	if prior2["mrr"] != 1500 {
		t.Fatalf("latest prior mrr = %v, want 1500", prior2["mrr"])
	}
}

func TestLoadPriorSnapshotCorruptBlob(t *testing.T) {
	db := newNovelTestStore(t)
	ctx := context.Background()

	// A row exists but its metrics_json is not valid JSON: this must be
	// reported as an error (deltas suppressed), NOT as a clean "first snapshot".
	if _, err := db.DB().ExecContext(ctx,
		`INSERT INTO rc_snapshots (project_id, captured_at, metrics_json) VALUES (?, ?, ?)`,
		"projX", time.Now().UTC().Format(time.RFC3339), "{not valid json",
	); err != nil {
		t.Fatalf("seed corrupt row: %v", err)
	}

	prior, _, hasPrior, err := loadPriorSnapshot(ctx, db, "projX")
	if err == nil {
		t.Fatal("expected an error for a corrupt prior-snapshot blob")
	}
	if hasPrior {
		t.Fatal("corrupt prior must not report hasPrior=true")
	}
	if len(prior) != 0 {
		t.Fatalf("corrupt prior must yield no metrics, got %+v", prior)
	}
}
