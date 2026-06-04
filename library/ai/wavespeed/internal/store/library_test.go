// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// TestArchiveAndLibraryHaveIndependentVersionDomains proves the two DBs are
// separate files migrated by separate version constants.
func TestArchiveAndLibraryHaveIndependentVersionDomains(t *testing.T) {
	dir := t.TempDir()
	archive, err := Open(filepath.Join(dir, "archive.db"))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archive.Close()
	lib, err := OpenLibrary(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	defer lib.Close()

	av, err := archive.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if av != StoreSchemaVersion {
		t.Fatalf("archive version = %d, want %d", av, StoreSchemaVersion)
	}
	lv, err := lib.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if lv != LibrarySchemaVersion {
		t.Fatalf("library version = %d, want %d", lv, LibrarySchemaVersion)
	}
}

// TestLibraryMigrationIsIdempotent re-opens the library DB and confirms the
// re-run is a clean no-op.
func TestLibraryMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	for i := 0; i < 3; i++ {
		s, err := OpenLibrary(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		s.Close()
	}
}

func seedGeneration(t *testing.T, s *Store, id, brand, platform, model, prompt string, cost float64) {
	t.Helper()
	if err := s.RecordGeneration(Generation{
		ID:             id,
		Command:        "pack",
		BrandName:      brand,
		BrandProfileID: brand,
		PlatformTarget: platform,
		ModelID:        model,
		Prompt:         prompt,
		Cost:           cost,
		Status:         "completed",
		Params:         json.RawMessage(`{"prompt":"` + prompt + `"}`),
	}); err != nil {
		t.Fatalf("record %s: %v", id, err)
	}
}

func TestLibraryRecordListSearchTagCost(t *testing.T) {
	s, err := OpenLibrary(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	seedGeneration(t, s, "g1", "helm", "instagram", "wavespeed-ai/flux-dev", "helm black hero shot", 1.0)
	seedGeneration(t, s, "g2", "helm", "tiktok", "wavespeed-ai/flux-dev", "helm black reel teaser", 2.0)
	seedGeneration(t, s, "g3", "other", "instagram", "wavespeed-ai/sdxl", "unrelated banana", 0.5)

	// List filter by brand.
	helm, err := s.ListGenerations(GenerationFilter{Brand: "helm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(helm) != 2 {
		t.Fatalf("brand=helm returned %d, want 2", len(helm))
	}

	// List filter by platform.
	ig, err := s.ListGenerations(GenerationFilter{Platform: "instagram"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ig) != 2 {
		t.Fatalf("platform=instagram returned %d, want 2", len(ig))
	}

	// FTS5 search.
	hits, err := s.SearchGenerations("helm", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("search 'helm' returned %d, want 2", len(hits))
	}

	// Malformed FTS query surfaces as an error (caller maps to usageErr).
	if _, err := s.SearchGenerations(`"unterminated`, 10); err == nil {
		t.Fatalf("expected FTS syntax error")
	}

	// Tagging.
	if err := s.AddTag("g1", "hero"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTag("g2", "hero"); err != nil {
		t.Fatal(err)
	}
	tagged, err := s.ListGenerations(GenerationFilter{Tag: "hero"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tagged) != 2 {
		t.Fatalf("tag=hero returned %d, want 2", len(tagged))
	}
	if err := s.RemoveTag("g1", "hero"); err != nil {
		t.Fatal(err)
	}
	tags, err := s.TagsFor("g1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Fatalf("g1 tags after remove = %v, want none", tags)
	}

	// GetGeneration surfaces tags.
	g2, err := s.GetGeneration("g2")
	if err != nil {
		t.Fatal(err)
	}
	if len(g2.Tags) != 1 || g2.Tags[0] != "hero" {
		t.Fatalf("g2 tags = %v", g2.Tags)
	}

	// Cost report grouped by brand.
	rows, err := s.CostReport(time.Time{}, "brand")
	if err != nil {
		t.Fatal(err)
	}
	var helmCost float64
	for _, r := range rows {
		if r.Key == "helm" {
			helmCost = r.TotalCost
		}
	}
	if helmCost != 3.0 {
		t.Fatalf("helm cost = %v, want 3.0", helmCost)
	}

	// Empty cost report (future date) yields zero rows, not an error.
	empty, err := s.CostReport(time.Now().Add(24*time.Hour), "model")
	if err != nil {
		t.Fatalf("empty cost report errored: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("future cost report = %v, want empty", empty)
	}
}

func TestBrandProfileUpsertGetList(t *testing.T) {
	s, err := OpenLibrary(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.UpsertBrandProfile("b1", "helm", json.RawMessage(`{"voice":"premium"}`)); err != nil {
		t.Fatal(err)
	}
	// Update preserves id, changes data.
	if _, err := s.UpsertBrandProfile("ignored", "helm", json.RawMessage(`{"voice":"bold"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBrandProfile("helm")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "b1" {
		t.Fatalf("id = %q, want b1 (preserved across update)", got.ID)
	}
	var body struct {
		Voice string `json:"voice"`
	}
	if err := json.Unmarshal(got.Data, &body); err != nil {
		t.Fatal(err)
	}
	if body.Voice != "bold" {
		t.Fatalf("voice = %q, want bold", body.Voice)
	}

	list, err := s.ListBrandProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("brand profiles = %d, want 1", len(list))
	}
}
