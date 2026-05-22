// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package harvard

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

func TestHarvardObjectToWork_HappyPath(t *testing.T) {
	r := harvardObject{
		ObjectID:             204498,
		ObjectNumber:         "1942.2.3",
		Title:                "Jade Implement",
		Dated:                "ca. 1500 BCE",
		DateBegin:            -1500,
		DateEnd:              -1400,
		Classification:       "Tools",
		Century:              "15th century BCE",
		Period:               "Shang dynasty",
		Culture:              "Chinese",
		Medium:               "Green stone",
		Technique:            "",
		PrimaryImageURL:      "https://nrs.harvard.edu/urn-3:HUAM:CARP09817_dynmc",
		BaseImageURL:         "https://nrs.harvard.edu/urn-3:HUAM:CARP09817",
		People:               []harvardPerson{{Name: "Workshop, Anyang", DisplayName: "Workshop of Anyang", Role: "Artist"}},
		URL:                  "https://harvardartmuseums.org/collections/object/204498",
		Copyright:            "",
		AccessLevel:          1,
		ImagePermissionLevel: 0,
	}
	w := harvardObjectToWork(r)

	cases := []struct{ name, got, want string }{
		{"ID", w.ID, "harvard:204498"},
		{"Source", w.Source, "harvard"},
		{"SourceID", w.SourceID, "204498"},
		{"Title", w.Title, "Jade Implement"},
		{"Creator", w.Creator, "Workshop of Anyang"},
		{"CreatorCanonical", w.CreatorCanonical, "workshop of anyang"},
		{"DateText", w.DateText, "ca. 1500 BCE"},
		{"Medium", w.Medium, "Green stone"},
		{"Classification", w.Classification, "Tools"},
		{"Period", w.Period, "Shang dynasty"},
		{"CultureRegion", w.CultureRegion, "China"},
		{"License", w.License, "Public domain (Harvard Art Museums)"},
		{"ImageURL", w.ImageURL, "https://nrs.harvard.edu/urn-3:HUAM:CARP09817_dynmc"},
		{"SourceURL", w.SourceURL, "https://harvardartmuseums.org/collections/object/204498"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if w.DateStart != -1500 || w.DateEnd != -1400 {
		t.Errorf("Date range = [%d,%d], want [-1500,-1400]", w.DateStart, w.DateEnd)
	}
	if w.Description != "" {
		t.Errorf("Description = %q, want empty (Harvard /object has no curator essay)", w.Description)
	}
	wantThumb := "https://nrs.harvard.edu/urn-3:HUAM:CARP09817/full/200,/0/default.jpg"
	if w.ThumbnailURL != wantThumb {
		t.Errorf("ThumbnailURL = %q, want %q", w.ThumbnailURL, wantThumb)
	}
}

func TestHarvardObjectToWork_PeriodFallsBackToCentury(t *testing.T) {
	r := harvardObject{
		ObjectID:        1,
		Title:           "Untitled",
		Century:         "18th century",
		Period:          "",
		PrimaryImageURL: "https://example.com/a.jpg",
	}
	w := harvardObjectToWork(r)
	if w.Period != "18th century" {
		t.Errorf("Period = %q, want %q (Century fallback)", w.Period, "18th century")
	}
}

func TestHarvardObjectToWork_MediumFallsBackToTechnique(t *testing.T) {
	r := harvardObject{
		ObjectID:        2,
		Title:           "Untitled",
		Medium:          "",
		Technique:       "Etching",
		PrimaryImageURL: "https://example.com/b.jpg",
	}
	w := harvardObjectToWork(r)
	if w.Medium != "Etching" {
		t.Errorf("Medium = %q, want %q (Technique fallback)", w.Medium, "Etching")
	}
}

func TestHarvardObjectToWork_CopyrightOverridesPublicDomainLicense(t *testing.T) {
	r := harvardObject{
		ObjectID:        3,
		Title:           "Modern Piece",
		Copyright:       "© 2020 Estate of the Artist",
		PrimaryImageURL: "https://example.com/c.jpg",
	}
	w := harvardObjectToWork(r)
	if w.License != "© 2020 Estate of the Artist" {
		t.Errorf("License = %q, want pass-through of copyright field", w.License)
	}
}

func TestPickCreator(t *testing.T) {
	cases := []struct {
		name   string
		people []harvardPerson
		want   string
	}{
		{
			name: "prefers Artist role over earlier non-Artist",
			people: []harvardPerson{
				{DisplayName: "Engraver Smith", Role: "Engraver"},
				{DisplayName: "Rembrandt van Rijn", Role: "Artist"},
			},
			want: "Rembrandt van Rijn",
		},
		{
			name: "Artist role case-insensitive",
			people: []harvardPerson{
				{DisplayName: "Hokusai", Role: "ARTIST"},
			},
			want: "Hokusai",
		},
		{
			name:   "falls back to people[0] when no Artist role",
			people: []harvardPerson{{DisplayName: "Workshop of X", Role: "Workshop"}},
			want:   "Workshop of X",
		},
		{
			name:   "empty array yields empty creator",
			people: nil,
			want:   "",
		},
		{
			name: "prefers DisplayName over Name when both present",
			people: []harvardPerson{
				{Name: "rembrandt van rijn", DisplayName: "Rembrandt van Rijn", Role: "Artist"},
			},
			want: "Rembrandt van Rijn",
		},
		{
			name: "falls back to Name when DisplayName empty",
			people: []harvardPerson{
				{Name: "Unknown Maker", DisplayName: "", Role: "Artist"},
			},
			want: "Unknown Maker",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickCreator(tc.people)
			if got != tc.want {
				t.Errorf("pickCreator() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCultureToRegion(t *testing.T) {
	cases := []struct{ culture, want string }{
		{"Japanese", "Japan"},
		{"Chinese", "China"},
		{"Korean", "Korea"},
		{"Indian", "India"},
		{"Egyptian", "Egypt"},
		{"Greek", "Mediterranean"},
		{"Roman", "Mediterranean"},
		{"Byzantine", "Mediterranean"},
		{"Islamic", "Islamic world"},
		{"Persian", "Islamic world"},
		{"Ottoman", "Islamic world"},
		{"Tibetan", "Himalaya"},
		{"Nepalese", "Himalaya"},
		{"American", "North America"},
		{"African", "Africa"},
		{"French", "Europe"},
		{"Italian", "Europe"},
		{"Dutch", "Europe"},
		{"Mexican", "Mesoamerica"},
		{"Mayan", "Mesoamerica"},
		{"", ""},
		{"Klingon", "Klingon"}, // unknown -> pass-through
	}
	for _, tc := range cases {
		t.Run(tc.culture, func(t *testing.T) {
			got := cultureToRegion(tc.culture)
			if got != tc.want {
				t.Errorf("cultureToRegion(%q) = %q, want %q", tc.culture, got, tc.want)
			}
		})
	}
}

func TestThumbnailURL(t *testing.T) {
	t.Run("constructs IIIF derivative from BaseImageURL", func(t *testing.T) {
		r := harvardObject{
			PrimaryImageURL: "https://nrs.harvard.edu/x_dynmc",
			BaseImageURL:    "https://nrs.harvard.edu/x",
		}
		got := thumbnailURL(r)
		want := "https://nrs.harvard.edu/x/full/200,/0/default.jpg"
		if got != want {
			t.Errorf("thumbnailURL = %q, want %q", got, want)
		}
	})
	t.Run("trims trailing slash on BaseImageURL", func(t *testing.T) {
		r := harvardObject{BaseImageURL: "https://nrs.harvard.edu/x/"}
		got := thumbnailURL(r)
		if !strings.Contains(got, "/x/full/") || strings.Contains(got, "x//full") {
			t.Errorf("thumbnailURL = %q; expected single slash between base and /full/", got)
		}
	})
	t.Run("falls back to PrimaryImageURL when BaseImageURL empty", func(t *testing.T) {
		r := harvardObject{
			PrimaryImageURL: "https://nrs.harvard.edu/x_dynmc",
			BaseImageURL:    "",
		}
		got := thumbnailURL(r)
		if got != r.PrimaryImageURL {
			t.Errorf("thumbnailURL = %q, want fallback to %q", got, r.PrimaryImageURL)
		}
	})
}

func TestSync_NoAPIKey_FailsFastWithSignupHint(t *testing.T) {
	// Both env-var paths the adapter checks must be unset.
	os.Unsetenv("ART_GOAT_HARVARD_KEY")
	os.Unsetenv("HARVARD_API_KEY")

	c := &Client{}
	_, err := c.Sync(context.Background(), source.SyncOpts{Limit: 5})
	if err == nil {
		t.Fatal("expected error when both API key env vars are unset")
	}
	msg := err.Error()
	for _, want := range []string{
		"HARVARD_API_KEY",
		"ART_GOAT_HARVARD_KEY",
		"docs.google.com/forms",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; got: %s", want, msg)
		}
	}
}
