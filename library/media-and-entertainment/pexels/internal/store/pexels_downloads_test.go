// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPexelsDownloadsRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	db := s.DB()

	if err := EnsurePexelsDownloads(db); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Idempotent.
	if err := EnsurePexelsDownloads(db); err != nil {
		t.Fatalf("ensure twice: %v", err)
	}

	rec := PexelsDownload{
		MediaID: 2014422, MediaType: "photo", Query: "mountain",
		Photographer: "Jane Doe", PhotographerURL: "https://pexels.com/@jane",
		PageURL: "https://pexels.com/photo/2014422", SrcURL: "https://images.pexels.com/x.jpg",
		FilePath: "/tmp/x.jpg", AvgColor: "#445566", Alt: "a mountain",
		DownloadedAt: "2026-06-23T00:00:00Z",
	}

	exists, err := PexelsDownloadExists(db, rec.MediaID, "photo")
	if err != nil {
		t.Fatalf("exists pre: %v", err)
	}
	if exists {
		t.Fatal("row should not exist before insert")
	}

	if err := InsertPexelsDownload(db, rec); err != nil {
		t.Fatalf("insert: %v", err)
	}

	exists, err = PexelsDownloadExists(db, rec.MediaID, "photo")
	if err != nil {
		t.Fatalf("exists post: %v", err)
	}
	if !exists {
		t.Fatal("row should exist after insert")
	}
	// Different media type does not collide.
	if exists, _ := PexelsDownloadExists(db, rec.MediaID, "video"); exists {
		t.Fatal("video row should not exist")
	}

	// INSERT OR REPLACE: re-insert with same key updates rather than dupes.
	rec.Photographer = "John Smith"
	if err := InsertPexelsDownload(db, rec); err != nil {
		t.Fatalf("reinsert: %v", err)
	}

	all, err := AllPexelsDownloads(db, nil)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len = %d, want 1 (INSERT OR REPLACE)", len(all))
	}
	if all[0].Photographer != "John Smith" {
		t.Errorf("photographer = %q, want John Smith", all[0].Photographer)
	}
	if all[0].AvgColor != "#445566" || all[0].Alt != "a mountain" {
		t.Errorf("nullable columns not preserved: %+v", all[0])
	}

	// Filter by type.
	vids, err := AllPexelsDownloads(db, []string{"video"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(vids) != 0 {
		t.Errorf("video filter len = %d, want 0", len(vids))
	}
	photos, err := AllPexelsDownloads(db, []string{"photo"})
	if err != nil {
		t.Fatalf("filter photo: %v", err)
	}
	if len(photos) != 1 {
		t.Errorf("photo filter len = %d, want 1", len(photos))
	}
}
