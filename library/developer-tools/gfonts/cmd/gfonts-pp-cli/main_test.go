package main

import "testing"

func TestCategoryKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"Sans Serif", "sansserif"},
		{"sans-serif", "sansserif"},
		{"sans_serif", "sansserif"},
		{"sans serif", "sansserif"},
		{"  SANS-SERIF  ", "sansserif"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := categoryKey(tc.in); got != tc.want {
			t.Errorf("categoryKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCategoriesMatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stored  string
		query   string
		want    bool
	}{
		{name: "hyphen slug matches stored display name", stored: "Sans Serif", query: "sans-serif", want: true},
		{name: "stored display name still matches", stored: "Sans Serif", query: "Sans Serif", want: true},
		{name: "space form matches stored display name", stored: "Sans Serif", query: "sans serif", want: true},
		{name: "underscore form matches stored display name", stored: "Sans Serif", query: "sans_serif", want: true},
		{name: "unrelated category does not match", stored: "Sans Serif", query: "serif", want: false},
		{name: "display single-token still matches", stored: "Display", query: "display", want: true},
		{name: "empty query does not match", stored: "Sans Serif", query: "", want: false},
		{name: "whitespace query does not match", stored: "Sans Serif", query: "   ", want: false},
		{name: "unknown slug does not match", stored: "Sans Serif", query: "not-a-category", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := categoriesMatch(tc.stored, tc.query); got != tc.want {
				t.Fatalf("categoriesMatch(%q, %q) = %v, want %v", tc.stored, tc.query, got, tc.want)
			}
		})
	}
}

func TestFilterByCategory(t *testing.T) {
	t.Parallel()
	meta := &FontMetadata{
		FamilyMetadataList: []Font{
			{Family: "Inter", Category: "Sans Serif"},
			{Family: "Playfair Display", Category: "Serif"},
			{Family: "Press Start 2P", Category: "Display"},
		},
	}
	cases := []struct {
		name     string
		query    string
		want     []string
	}{
		{name: "hyphen slug matches fonts with Category Sans Serif", query: "sans-serif", want: []string{"Inter"}},
		{name: "stored display name still matches", query: "Sans Serif", want: []string{"Inter"}},
		{name: "space form matches", query: "sans serif", want: []string{"Inter"}},
		{name: "unrelated category does not match", query: "handwriting", want: nil},
		{name: "empty slug returns empty", query: "", want: nil},
		{name: "unknown slug returns empty", query: "not-a-category", want: nil},
		{name: "single-token display still matches", query: "display", want: []string{"Press Start 2P"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterByCategory(meta, tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("filterByCategory(%q) returned %d fonts %v, want %v", tc.query, len(got), families(got), tc.want)
			}
			for i, family := range tc.want {
				if got[i].Family != family {
					t.Fatalf("filterByCategory(%q)[%d] = %q, want %q", tc.query, i, got[i].Family, family)
				}
			}
		})
	}
}

func families(fonts []Font) []string {
	out := make([]string, len(fonts))
	for i, f := range fonts {
		out[i] = f.Family
	}
	return out
}
