// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
)

func TestSEOSnapshots(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureSEOSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	old := seohead.Head{URL: "https://x.test/", FinalURL: "https://x.test/final", FetchedAt: base, Status: 200, Title: "Old", OGTitle: "Open Graph", OGDescription: "OG desc", SchemaTypes: []string{"Article"}}
	newer := old
	newer.FetchedAt = base.Add(time.Hour)
	newer.Title = "New"
	newer.Noindex = true
	failed := newer
	failed.FetchedAt = base.Add(2 * time.Hour)
	failed.Status = 500
	failed.Title = "Failed fetch"
	if err = s.InsertSEOSnapshots(ctx, []seohead.Head{old, newer, failed}); err != nil {
		t.Fatal(err)
	}
	latest, err := s.LatestSEOSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || latest[0].Title != "New" || !latest[0].Noindex {
		t.Fatalf("%+v", latest)
	}
	previous, err := s.PreviousSEOSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prev, ok := previous[old.URL]
	if !ok || prev.Title != "Old" || prev.FinalURL != old.FinalURL || prev.OGTitle != old.OGTitle || prev.OGDescription != old.OGDescription {
		t.Fatalf("%+v %v", prev, ok)
	}
}

func TestEnsureSEOSnapshotsAddsColumns(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE seo_snapshots (url TEXT NOT NULL, fetched_at TEXT NOT NULL, status INT, title TEXT, description TEXT, canonical TEXT, robots TEXT, noindex INT, schema_types TEXT, PRIMARY KEY(url, fetched_at))`); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSEOSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	h := seohead.Head{URL: "https://old.test/", FinalURL: "https://old.test/final", FetchedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), OGTitle: "OG", OGDescription: "Description", SchemaTypes: make([]string, 0)}
	if err := s.InsertSEOSnapshot(ctx, h); err != nil {
		t.Fatal(err)
	}
	rows, err := s.LatestSEOSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].FinalURL != h.FinalURL || rows[0].OGTitle != h.OGTitle || rows[0].OGDescription != h.OGDescription {
		t.Fatalf("migrated snapshot = %+v", rows)
	}
}

// Rows written before final_url/og_* existed are NULL in those columns after migration.
func TestSEOSnapshotsLegacyNullColumns(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE seo_snapshots(url TEXT, fetched_at TEXT, status INT, title TEXT, description TEXT, canonical TEXT, robots TEXT, noindex INT, schema_types TEXT, PRIMARY KEY(url, fetched_at))`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO seo_snapshots VALUES('https://x.test/','2026-01-01T00:00:00.000000000Z',200,'T','','','',0,'[]')`); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSEOSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	heads, err := s.LatestSEOSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(heads) != 1 || heads[0].Title != "T" || heads[0].FinalURL != "" {
		t.Fatalf("legacy row = %+v", heads)
	}
}
