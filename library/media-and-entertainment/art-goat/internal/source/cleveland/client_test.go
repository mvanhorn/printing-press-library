// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cleveland

import (
	"testing"
)

func TestClevelandArtworkToWork_HappyPath(t *testing.T) {
	a := clevelandArtwork{
		ID:              141261,
		AccessionNumber: "1916.1335",
		Title:           "The Stag Beetle",
		Creators: []clevelandCreator{
			{Description: "Albrecht Durer (German, 1471-1528)"},
		},
		CreationDate:         "1505",
		CreationDateEarliest: 1505,
		CreationDateLatest:   1505,
		Technique:            "watercolor and gouache",
		Type:                 "Drawing",
		Department:           "Drawings",
		Culture:              []string{"Germany, 16th century"},
		Description:          "A meticulous nature study.",
		Images: clevelandImages{
			Web:   clevelandImageVariant{URL: "https://openaccess-cdn.clevelandart.org/1916.1335/1916.1335_web.jpg"},
			Print: clevelandImageVariant{URL: "https://openaccess-cdn.clevelandart.org/1916.1335/1916.1335_print.jpg"},
		},
		URL: "https://www.clevelandart.org/art/1916.1335",
	}
	w := clevelandArtworkToWork(a)

	cases := []struct {
		name, got, want string
	}{
		{"ID", w.ID, "cleveland:141261"},
		{"Source", w.Source, "cleveland"},
		{"SourceID", w.SourceID, "141261"},
		{"Title", w.Title, "The Stag Beetle"},
		{"Creator", w.Creator, "Albrecht Durer (German, 1471-1528)"},
		{"CreatorCanonical", w.CreatorCanonical, "albrecht durer (german, 1471-1528)"},
		{"DateText", w.DateText, "1505"},
		{"Medium", w.Medium, "watercolor and gouache"},
		{"Classification", w.Classification, "Drawing"},
		{"Period", w.Period, "Drawings"},
		{"CultureRegion", w.CultureRegion, "Germany, 16th century"},
		{"Description", w.Description, "A meticulous nature study."},
		{"ImageURL", w.ImageURL, "https://openaccess-cdn.clevelandart.org/1916.1335/1916.1335_web.jpg"},
		{"ThumbnailURL", w.ThumbnailURL, "https://openaccess-cdn.clevelandart.org/1916.1335/1916.1335_print.jpg"},
		{"License", w.License, "CC0"},
		{"SourceURL", w.SourceURL, "https://www.clevelandart.org/art/1916.1335"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if w.DateStart != 1505 || w.DateEnd != 1505 {
		t.Errorf("Date range = [%d,%d], want [1505,1505]", w.DateStart, w.DateEnd)
	}
	if w.RawJSON == "" {
		t.Error("RawJSON should be populated")
	}
	if w.SyncedAt.IsZero() {
		t.Error("SyncedAt should be set")
	}
}

func TestClevelandArtworkToWork_ThumbnailFallback(t *testing.T) {
	// When Print URL is empty the thumbnail falls back to the Web URL.
	a := clevelandArtwork{
		ID:    1,
		Title: "T",
		Images: clevelandImages{
			Web:   clevelandImageVariant{URL: "https://example.com/web.jpg"},
			Print: clevelandImageVariant{URL: ""},
		},
	}
	w := clevelandArtworkToWork(a)
	if w.ThumbnailURL != "https://example.com/web.jpg" {
		t.Errorf("ThumbnailURL = %q, want web fallback", w.ThumbnailURL)
	}
}

func TestClevelandArtworkToWork_MissingImage(t *testing.T) {
	// Cleveland sync filters by has_image=1, but a record could still
	// arrive with both URLs blank — the mapper itself does not skip it
	// (the call site does), so document the empty-image behavior.
	a := clevelandArtwork{
		ID:    42,
		Title: "Untitled",
	}
	w := clevelandArtworkToWork(a)
	if w.ImageURL != "" || w.ThumbnailURL != "" {
		t.Errorf("expected empty image URLs, got web=%q print=%q", w.ImageURL, w.ThumbnailURL)
	}
	if w.ID != "cleveland:42" {
		t.Errorf("ID = %q, want cleveland:42", w.ID)
	}
}

func TestClevelandArtworkToWork_ZeroIDSkipped(t *testing.T) {
	// Sync uses w.ID == "" as a skip signal; an ID of 0 must produce the
	// zero Work so the loop drops it.
	w := clevelandArtworkToWork(clevelandArtwork{ID: 0, Title: "Ghost"})
	if w.ID != "" {
		t.Errorf("expected zero Work for ID=0, got ID=%q", w.ID)
	}
}

func TestClevelandArtworkToWork_MissingCreator(t *testing.T) {
	// Cleveland's Creators array is often empty (anonymous artisans,
	// archaeological items). Creator must fall back to empty string.
	a := clevelandArtwork{
		ID:       7,
		Title:    "Bronze vessel",
		Creators: nil,
		Images: clevelandImages{
			Web: clevelandImageVariant{URL: "https://example.com/v.jpg"},
		},
	}
	w := clevelandArtworkToWork(a)
	if w.Creator != "" {
		t.Errorf("Creator = %q, want empty", w.Creator)
	}
	if w.CreatorCanonical != "" {
		t.Errorf("CreatorCanonical = %q, want empty", w.CreatorCanonical)
	}
}

func TestClevelandArtworkToWork_EmptyCultureArray(t *testing.T) {
	a := clevelandArtwork{
		ID:      9,
		Title:   "X",
		Culture: nil,
		Images:  clevelandImages{Web: clevelandImageVariant{URL: "https://example.com/x.jpg"}},
	}
	w := clevelandArtworkToWork(a)
	if w.CultureRegion != "" {
		t.Errorf("CultureRegion = %q, want empty", w.CultureRegion)
	}
}
