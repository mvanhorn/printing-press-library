// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEnsureKDPSchemaAndSnapshots(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "kdp.db")
	s, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.EnsureKDPSchema(ctx); err != nil {
		t.Fatalf("EnsureKDPSchema: %v", err)
	}
	// Idempotent re-run.
	if err := s.EnsureKDPSchema(ctx); err != nil {
		t.Fatalf("EnsureKDPSchema (rerun): %v", err)
	}

	// bucket column exists.
	rows, err := s.DB().QueryContext(ctx, `PRAGMA table_info("niches")`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	hasBucket := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			t.Fatalf("scan: %v", err)
		}
		if name == "bucket" {
			hasBucket = true
		}
	}
	rows.Close()
	if !hasBucket {
		t.Fatalf("niches table missing bucket column")
	}

	if err := s.UpsertNiche(2584, "TAI CHI", "https://www.amazon.com/dp/B0GTDWD9QL", "", "19.99", "Indie", 271, 5422.71, "hidden_gems"); err != nil {
		t.Fatalf("UpsertNiche: %v", err)
	}
	var bucket string
	if err := s.DB().QueryRow(`SELECT bucket FROM niches WHERE id = ?`, "2584").Scan(&bucket); err != nil {
		t.Fatalf("read bucket: %v", err)
	}
	if bucket != "hidden_gems" {
		t.Fatalf("got bucket %q, want hidden_gems", bucket)
	}

	// Two snapshots on different dates; same-day overwrites.
	if err := s.RecordSnapshot("2584", "hidden_gems", "2026-06-01", 200, 4000.0, "19.99", "TAI CHI"); err != nil {
		t.Fatalf("snapshot 1: %v", err)
	}
	if err := s.RecordSnapshot("2584", "hidden_gems", "2026-06-15", 271, 5422.71, "19.99", "TAI CHI"); err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}
	if err := s.RecordSnapshot("2584", "hidden_gems", "2026-06-15", 300, 6000.0, "19.99", "TAI CHI"); err != nil {
		t.Fatalf("snapshot 2 overwrite: %v", err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM niche_snapshots WHERE book_id = ?`, "2584").Scan(&count); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 2 {
		t.Fatalf("got %d snapshots, want 2 (same-day should overwrite)", count)
	}
	var rev float64
	if err := s.DB().QueryRow(`SELECT estimated_monthly_revenue FROM niche_snapshots WHERE book_id = ? AND captured_on = ?`, "2584", "2026-06-15").Scan(&rev); err != nil {
		t.Fatalf("read overwritten snapshot: %v", err)
	}
	if rev != 6000.0 {
		t.Fatalf("got revenue %v, want 6000 (overwrite)", rev)
	}
}
