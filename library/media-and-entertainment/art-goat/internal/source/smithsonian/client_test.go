// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package smithsonian

import (
	"encoding/json"
	"testing"
)

func TestRowToWork_HappyPath(t *testing.T) {
	row := siRow{
		ID:       "edanmdm-saam_1968.155.8",
		Title:    "Migration",
		UnitCode: "SAAM",
		Content: &siContent{
			DescriptiveNonRepeating: &siDescNonRepeating{
				Title:    &siLabeledContent{Label: "Title", Content: "Migration"},
				GUID:     "https://americanart.si.edu/artwork/migration-65535",
				RecordID: "saam_1968.155.8",
				OnlineMedia: &siOnlineMedia{
					MediaCount: 1,
					Media: []siMedia{
						{
							Content:   "https://ids.si.edu/full/migration.jpg",
							Thumbnail: "https://ids.si.edu/thumb/migration.jpg",
							Type:      "Images",
						},
					},
				},
			},
			IndexedStructured: &siIndexedStructured{
				Date:       json.RawMessage(`"1942"`),
				ObjectType: json.RawMessage(`["Painting"]`),
				Culture:    json.RawMessage(`["American"]`),
			},
			Freetext: &siFreetext{
				Name: []siLabeledContent{
					{Label: "Artist", Content: "Jacob Lawrence"},
				},
			},
		},
	}
	w := rowToWork(row)

	cases := []struct {
		name, got, want string
	}{
		{"ID", w.ID, "smithsonian:edanmdm-saam_1968.155.8"},
		{"Source", w.Source, "smithsonian"},
		{"SourceID", w.SourceID, "edanmdm-saam_1968.155.8"},
		{"Title", w.Title, "Migration"},
		{"Creator", w.Creator, "Jacob Lawrence"},
		{"CreatorCanonical", w.CreatorCanonical, "jacob lawrence"},
		{"DateText", w.DateText, "1942"},
		{"Medium", w.Medium, "Painting"},
		{"CultureRegion", w.CultureRegion, "American"},
		{"License", w.License, "CC0"},
		{"SourceURL", w.SourceURL, "https://americanart.si.edu/artwork/migration-65535"},
		{"ImageURL", w.ImageURL, "https://ids.si.edu/full/migration.jpg"},
		{"ThumbnailURL", w.ThumbnailURL, "https://ids.si.edu/thumb/migration.jpg"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	// 4-digit year regex pulls 1942 into both DateStart and DateEnd.
	if w.DateStart != 1942 || w.DateEnd != 1942 {
		t.Errorf("Date range = [%d,%d], want [1942,1942]", w.DateStart, w.DateEnd)
	}
}

func TestRowToWork_EmptyIDSkipped(t *testing.T) {
	// rowToWork must return the zero Work when ID is missing — Sync
	// uses that as the skip signal.
	w := rowToWork(siRow{ID: "   "})
	if w.ID != "" {
		t.Errorf("expected zero Work for blank ID, got ID=%q", w.ID)
	}
}

func TestRowToWork_MissingImage(t *testing.T) {
	// No OnlineMedia block -> ImageURL stays empty. Sync drops this.
	row := siRow{
		ID:    "x-1",
		Title: "Noisy",
		Content: &siContent{
			DescriptiveNonRepeating: &siDescNonRepeating{
				Title: &siLabeledContent{Content: "Noisy"},
				GUID:  "https://example.com/x-1",
			},
			Freetext: &siFreetext{Name: []siLabeledContent{{Content: "Maker"}}},
		},
	}
	w := rowToWork(row)
	if w.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty", w.ImageURL)
	}
	if w.ID != "smithsonian:x-1" {
		t.Errorf("ID = %q, want smithsonian:x-1", w.ID)
	}
}

func TestRowToWork_MissingCreator(t *testing.T) {
	// No Freetext.Name entries -> Creator stays empty (Smithsonian
	// convention: do not synthesize "Unknown").
	row := siRow{
		ID:    "y-1",
		Title: "Anonymous",
		Content: &siContent{
			DescriptiveNonRepeating: &siDescNonRepeating{
				GUID: "https://example.com/y-1",
				OnlineMedia: &siOnlineMedia{
					Media: []siMedia{{Content: "https://example.com/y.jpg"}},
				},
			},
			Freetext: nil,
		},
	}
	w := rowToWork(row)
	if w.Creator != "" {
		t.Errorf("Creator = %q, want empty", w.Creator)
	}
	if w.CreatorCanonical != "" {
		t.Errorf("CreatorCanonical = %q, want empty", w.CreatorCanonical)
	}
}

func TestRowToWork_NilContent(t *testing.T) {
	// Defensive: an id-only row with no content shouldn't panic.
	w := rowToWork(siRow{ID: "z-1", Title: "Bare"})
	if w.ID != "smithsonian:z-1" {
		t.Errorf("ID = %q, want smithsonian:z-1", w.ID)
	}
	if w.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty", w.ImageURL)
	}
}

// firstString covers the 4 polymorphic shapes Smithsonian's
// indexedStructured fields can take, plus the null/missing fallback.
func TestFirstString_PolymorphicShapes(t *testing.T) {
	cases := []struct {
		name string
		in   json.RawMessage
		want string
	}{
		{"empty", json.RawMessage(``), ""},
		{"null", json.RawMessage(`null`), ""},
		{"bare string", json.RawMessage(`"Painting"`), "Painting"},
		{"string trimmed", json.RawMessage(`"  Painting  "`), "Painting"},
		{"array of strings", json.RawMessage(`["Painting", "Oil on canvas"]`), "Painting"},
		{"array of strings skips blanks", json.RawMessage(`["", "  ", "Sculpture"]`), "Sculpture"},
		{"array of labeled objects", json.RawMessage(`[{"label":"Type","content":"Drawing"}]`), "Drawing"},
		{"array of labeled objects skips blanks", json.RawMessage(`[{"content":""},{"content":"Print"}]`), "Print"},
		{"single labeled object", json.RawMessage(`{"label":"Type","content":"Photograph"}`), "Photograph"},
		{"single labeled object empty content", json.RawMessage(`{"label":"Type","content":""}`), ""},
		{"unrecognized leading char", json.RawMessage(`42`), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstString(tc.in)
			if got != tc.want {
				t.Errorf("firstString(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
