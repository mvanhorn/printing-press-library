// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package met

import (
	"encoding/json"
	"strings"
	"testing"
)

// fixtureMetHokusai is a realistic /objects/{id} payload shape pulled from
// the documented Met Collection API response. Only the fields the mapper
// reads are populated; RawJSON in the test asserts on the literal string.
const fixtureMetHokusai = `{
  "objectID": 45434,
  "title": "Under the Wave off Kanagawa (Kanagawa oki nami ura), also known as The Great Wave",
  "artistDisplayName": "Katsushika Hokusai",
  "objectDate": "ca. 1830-32",
  "objectBeginDate": 1830,
  "objectEndDate": 1832,
  "medium": "Polychrome woodblock print; ink and color on paper",
  "classification": "Prints",
  "period": "Edo period (1615-1868)",
  "culture": "Japan",
  "country": "Japan",
  "primaryImage": "https://images.metmuseum.org/CRDImages/as/original/DP141538.jpg",
  "primaryImageSmall": "https://images.metmuseum.org/CRDImages/as/web-large/DP141538.jpg",
  "objectURL": "https://www.metmuseum.org/art/collection/search/45434",
  "isPublicDomain": true
}`

func decodeMetFixture(t *testing.T, raw string) metObject {
	t.Helper()
	var obj metObject
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return obj
}

func TestMetObjectToWork_HappyPath(t *testing.T) {
	obj := decodeMetFixture(t, fixtureMetHokusai)
	w := metObjectToWork(obj, fixtureMetHokusai)

	cases := []struct {
		name, got, want string
	}{
		{"ID", w.ID, "met:45434"},
		{"Source", w.Source, "met"},
		{"SourceID", w.SourceID, "45434"},
		{"Title", w.Title, "Under the Wave off Kanagawa (Kanagawa oki nami ura), also known as The Great Wave"},
		{"Creator", w.Creator, "Katsushika Hokusai"},
		{"CreatorCanonical", w.CreatorCanonical, "katsushika hokusai"},
		{"DateText", w.DateText, "ca. 1830-32"},
		{"Medium", w.Medium, "Polychrome woodblock print; ink and color on paper"},
		{"Classification", w.Classification, "Prints"},
		{"Period", w.Period, "Edo period (1615-1868)"},
		{"CultureRegion", w.CultureRegion, "Japan"},
		{"ImageURL", w.ImageURL, "https://images.metmuseum.org/CRDImages/as/original/DP141538.jpg"},
		{"ThumbnailURL", w.ThumbnailURL, "https://images.metmuseum.org/CRDImages/as/web-large/DP141538.jpg"},
		{"License", w.License, "CC0"},
		{"SourceURL", w.SourceURL, "https://www.metmuseum.org/art/collection/search/45434"},
		{"RawJSON", w.RawJSON, fixtureMetHokusai},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if w.DateStart != 1830 {
		t.Errorf("DateStart = %d, want 1830", w.DateStart)
	}
	if w.DateEnd != 1832 {
		t.Errorf("DateEnd = %d, want 1832", w.DateEnd)
	}
	if w.SyncedAt.IsZero() {
		t.Error("SyncedAt should be set")
	}
}

func TestMetObjectToWork_CountryFallback(t *testing.T) {
	// When Culture is empty, region falls back to Country.
	obj := metObject{
		ObjectID:     1,
		Title:        "T",
		Culture:      "",
		Country:      "France",
		PrimaryImage: "https://example.com/x.jpg",
	}
	w := metObjectToWork(obj, "{}")
	if w.CultureRegion != "France" {
		t.Errorf("CultureRegion = %q, want %q", w.CultureRegion, "France")
	}
}

func TestMetObjectToWork_NonPublicDomainLicense(t *testing.T) {
	obj := metObject{ObjectID: 1, IsPublicDomain: false}
	w := metObjectToWork(obj, "{}")
	if w.License != "Met collection terms" {
		t.Errorf("License = %q, want %q", w.License, "Met collection terms")
	}
}

func TestMetObjectToWork_MissingImage(t *testing.T) {
	// Mapper itself emits a Work with empty ImageURL; the Sync loop is
	// responsible for skipping such records. Document that contract here.
	obj := metObject{
		ObjectID:          77,
		Title:             "Untitled",
		ArtistDisplayName: "A",
		PrimaryImage:      "",
	}
	w := metObjectToWork(obj, "{}")
	if w.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty", w.ImageURL)
	}
	if w.ID != "met:77" {
		t.Errorf("ID = %q, want met:77", w.ID)
	}
}

func TestMetObjectToWork_MissingCreator(t *testing.T) {
	// Met records frequently have no ArtistDisplayName (anonymous,
	// unattributed). Creator must fall back to "" cleanly, and the
	// canonical form must follow suit.
	obj := metObject{
		ObjectID:          88,
		Title:             "Bowl",
		ArtistDisplayName: "   ",
		PrimaryImage:      "https://example.com/y.jpg",
	}
	w := metObjectToWork(obj, "{}")
	if w.Creator != "" {
		t.Errorf("Creator = %q, want empty", w.Creator)
	}
	if w.CreatorCanonical != "" {
		t.Errorf("CreatorCanonical = %q, want empty", w.CreatorCanonical)
	}
	// Trimming applied consistently.
	if strings.TrimSpace(w.Creator) != "" {
		t.Errorf("Creator not trimmed: %q", w.Creator)
	}
}
