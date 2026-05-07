package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.sqlite")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	// Re-opening should be a no-op (idempotent migrations).
	s2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	_ = s2.Close()
}

func TestUpsertAnalytics_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows := []AnalyticsRow{
		{SiteURL: "sc-domain:example.com", SearchType: "web", Date: "2026-01-01",
			Query: "test", Page: "https://example.com/", Country: "usa", Device: "desktop",
			Clicks: 5, Impressions: 100, CTR: 0.05, Position: 12.3},
	}
	n, err := s.UpsertAnalytics(context.Background(), rows)
	if err != nil || n != 1 {
		t.Fatalf("first upsert: n=%d err=%v", n, err)
	}
	// Update clicks; primary key should collide → REPLACE.
	rows[0].Clicks = 7
	n, err = s.UpsertAnalytics(context.Background(), rows)
	if err != nil || n != 1 {
		t.Fatalf("second upsert: n=%d err=%v", n, err)
	}
	var got float64
	if err := s.DB().QueryRow(`SELECT clicks FROM search_analytics_rows`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Errorf("clicks=%v, want 7", got)
	}
}

func TestSnapshotSites_AndSitemaps(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.SnapshotSites(context.Background(), now,
		[]SiteRow{{SiteURL: "sc-domain:example.com", PermissionLevel: "siteOwner"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SnapshotSitemaps(context.Background(), now, []SitemapRow{{
		SiteURL: "sc-domain:example.com", FeedPath: "https://example.com/sitemap.xml",
		LastSubmitted: "2026-01-01T00:00:00Z", Errors: 0, Warnings: 2,
	}}); err != nil {
		t.Fatal(err)
	}
	var sites, smaps int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM sites_snapshots`).Scan(&sites)
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM sitemaps_snapshots`).Scan(&smaps)
	if sites != 1 || smaps != 1 {
		t.Errorf("sites=%d smaps=%d", sites, smaps)
	}
}

func TestSaveURLInspection_HistoryPreserved(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := URLInspectionRow{
		InspectedAt:   "2026-01-01T00:00:00Z",
		SiteURL:       "sc-domain:example.com",
		PageURL:       "https://example.com/a",
		CoverageState: "INDEXED",
	}
	if err := s.SaveURLInspection(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	r2 := r
	r2.InspectedAt = "2026-01-08T00:00:00Z"
	r2.CoverageState = "URL_IS_NOT_ON_GOOGLE"
	if err := s.SaveURLInspection(context.Background(), r2); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM url_inspections WHERE page_url = ?`, r.PageURL).Scan(&n)
	if n != 2 {
		t.Errorf("history rows=%d, want 2", n)
	}
}
