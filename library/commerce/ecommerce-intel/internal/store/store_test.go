package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadProfileAndData(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", ShopifyShop: "demo.myshopify.com", GAProperty: "123"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.ShopifyShop != "demo.myshopify.com" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
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
	if len(loaded.Products) != 3 || len(loaded.Pages) != 2 || loaded.Source != "embedded-shopify-commerce-fixture" {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	if !loaded.Storefront.StructuredData || loaded.Storefront.Answerability == 0 {
		t.Fatalf("missing GEO fixture fields: %#v", loaded.Storefront)
	}
	for _, p := range loaded.Products {
		if p.Handle == "" || p.Revenue == 0 {
			t.Fatalf("bad product fixture: %#v", p)
		}
		if !p.Source.Shopify.Synced || !p.Source.GA4.Synced || !p.Source.GSC.Synced || !p.Source.Ahrefs.Synced || !p.Source.Klaviyo.Synced {
			t.Fatalf("source evidence not mirrored: %#v", p.Source)
		}
	}
	for _, p := range loaded.Pages {
		if !p.Source.GA4.Synced || !p.Source.GSC.Synced || !p.Source.Ahrefs.Synced {
			t.Fatalf("page source evidence missing: %#v", p.Source)
		}
	}
	for _, c := range loaded.Categories {
		if !c.Source.Shopify.Synced || !c.Source.GA4.Synced {
			t.Fatalf("category source evidence missing: %#v", c.Source)
		}
	}
	for _, e := range loaded.Emails {
		if !e.Source.Klaviyo.Synced {
			t.Fatalf("email source evidence missing: %#v", e.Source)
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
