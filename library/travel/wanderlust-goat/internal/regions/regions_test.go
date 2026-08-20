package regions

import (
	"testing"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		wantPrim  string
		wantSites bool // some review sites
	}{
		{"japan", "JP", "ja", true},
		{"japan-lower", "jp", "ja", true},
		{"korea", "KR", "ko", true},
		{"france", "FR", "fr", true},
		{"germany", "DE", "de", true},
		{"austria-grouped-with-DE", "AT", "de", true},
		{"switzerland-grouped-with-DE", "CH", "de", true},
		{"uk", "GB", "en", true},
		{"ireland-grouped-with-UK", "IE", "en", true},
		{"china-no-google", "CN", "zh", true},
		{"unknown", "ZZ", "en", false},
		{"empty", "", "en", false},
		{"universal", "*", "en", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Lookup(tt.code)
			if r.PrimaryLanguage != tt.wantPrim {
				t.Errorf("PrimaryLanguage = %q, want %q", r.PrimaryLanguage, tt.wantPrim)
			}
			gotSites := len(r.LocalReviewSites) > 0
			if gotSites != tt.wantSites {
				t.Errorf("LocalReviewSites populated = %v, want %v", gotSites, tt.wantSites)
			}
		})
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) < 8 {
		t.Errorf("All() returned %d regions, expected >=8 (JP/KR/CN/FR/IT/DACH/ES/UK)", len(all))
	}
}

func TestAllSourceSlugs_NoDupes(t *testing.T) {
	slugs := AllSourceSlugs()
	seen := map[string]bool{}
	for _, s := range slugs {
		if seen[s] {
			t.Errorf("duplicate slug %q in AllSourceSlugs()", s)
		}
		seen[s] = true
	}
	if len(slugs) < 18 {
		t.Errorf("AllSourceSlugs() returned %d, expected >=18 (sum of LocalReviewSites across regions)", len(slugs))
	}
}

func TestChinaHasNoGoogleTLD(t *testing.T) {
	r := Lookup("CN")
	if r.GoogleTLD != "" {
		t.Errorf("CN GoogleTLD = %q, want empty (Google blocked in CN)", r.GoogleTLD)
	}
}

func TestFallbackHasTravelForums(t *testing.T) {
	fb := Fallback()
	if len(fb.LocalForums) == 0 {
		t.Error("Fallback should have at least one general travel forum")
	}
	if len(fb.LocalReviewSites) > 0 {
		t.Errorf("Fallback should have no review sites, got %v", fb.LocalReviewSites)
	}
}
