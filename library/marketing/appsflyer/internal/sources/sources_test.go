package sources

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"canonical exact", "facebook_int", "facebook_int", true},
		{"canonical case-insensitive", "FACEBOOK_INT", "facebook_int", true},
		{"display exact", "Meta ads", "facebook_int", true},
		{"alias short", "fb", "facebook_int", true},
		{"alias multichar", "tiktok", "tiktokglobal_int", true},
		{"google alias", "google", "googleadwords_int", true},
		{"display substring", "Reddit", "reddit_int", true},
		{"empty input", "", "", false},
		{"unknown", "definitelynotasource", "", false},
		{"too short for substring", "ti", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Resolve(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("Resolve(%q) = %q,%v; want %q,%v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantMin     int    // at least this many matches
		wantInclude string // canonical that must appear in results
	}{
		{"empty returns full catalog", "", 10, "facebook_int"},
		{"tiktok matches", "tiktok", 1, "tiktokglobal_int"},
		{"meta matches via display", "Meta", 1, "facebook_int"},
		{"alias substring", "fb", 1, "facebook_int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Search(tt.query)
			if len(got) < tt.wantMin {
				t.Fatalf("Search(%q): got %d results, want >=%d", tt.query, len(got), tt.wantMin)
			}
			found := false
			for _, s := range got {
				if s.Canonical == tt.wantInclude {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Search(%q): results did not include %q", tt.query, tt.wantInclude)
			}
		})
	}
}

func TestCatalogIsNonEmptyAndUnique(t *testing.T) {
	c := Catalog()
	if len(c) < 50 {
		t.Fatalf("Catalog returned %d entries, expected >= 50", len(c))
	}
	seen := make(map[string]bool, len(c))
	for _, s := range c {
		if seen[s.Canonical] {
			t.Fatalf("duplicate canonical %q in Catalog", s.Canonical)
		}
		seen[s.Canonical] = true
		if s.Canonical == "" || s.Display == "" {
			t.Fatalf("empty canonical or display in entry %+v", s)
		}
	}
}
