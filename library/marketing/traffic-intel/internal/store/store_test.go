package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadProfileAndData(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", SiteURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.SiteURL != "https://example.com" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatalf("bad profile: %#v", p)
	}
	d := Fixture("demo")
	if err := s.SaveData(d); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadData("demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Pages) != 4 || loaded.Source != "embedded-fixture" {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	for _, p := range loaded.Pages {
		if p.URL == "/best-blue-ray-ripper" {
			t.Fatalf("fixture contains unrelated page: %#v", p)
		}
		if p.Sources.GSC.Clicks != p.Clicks || p.Sources.GA4.Revenue != p.Revenue || p.Sources.Ahrefs.RefDomains != p.RefDomains {
			t.Fatalf("source fields not mirrored for %s: %#v", p.URL, p.Sources)
		}
	}
	snaps, err := s.LatestSnapshots("demo", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].SchemaVersion != SnapshotSchemaVersion || snaps[0].Profile != "demo" {
		t.Fatalf("snapshot not written with schema/profile: %#v", snaps)
	}
	if _, err := os.Stat(s.LearningsPath("demo")); err != nil {
		t.Fatalf("learnings file not created: %v", err)
	}
}

func TestSnapshotRetentionCompactsDailyAndWeekly(t *testing.T) {
	s := New(t.TempDir())
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	for _, ts := range []time.Time{
		now.AddDate(0, 0, -1).Add(time.Hour),
		now.AddDate(0, 0, -1).Add(2 * time.Hour),
		now.AddDate(0, 0, -35),
		now.AddDate(0, 0, -36),
		now.AddDate(0, 0, -43),
	} {
		if err := s.SaveSnapshot(DataSet{Profile: "demo", SyncedAt: ts, Source: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CompactSnapshots("demo", now); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(s.Dir, "snapshots", "demo", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected daily plus weekly compacted snapshots, got %d: %#v", len(paths), paths)
	}
}
