package sources

import (
	"math"
	"testing"
)

func TestBySlug(t *testing.T) {
	tests := []struct {
		slug    string
		wantNil bool
	}{
		{"nominatim", false},
		{"tabelog", false},
		{"reddit", false},
		{"unknown-source", true},
		{"", true},
	}
	for _, tc := range tests {
		got := BySlug(tc.slug)
		if (got == nil) != tc.wantNil {
			t.Errorf("BySlug(%q): got nil=%v, want nil=%v", tc.slug, got == nil, tc.wantNil)
		}
	}
}

func TestForCountryIncludesUniversalAndCanonical(t *testing.T) {
	got := ForCountry(CountryJapan)
	hasNominatim := false
	hasTabelog := false
	hasLeFooding := false
	for _, s := range got {
		if s.Slug == "nominatim" {
			hasNominatim = true
		}
		if s.Slug == "tabelog" {
			hasTabelog = true
		}
		if s.Slug == "lefooding" {
			hasLeFooding = true
		}
	}
	if !hasNominatim {
		t.Error("ForCountry(JP) should include Nominatim (universal)")
	}
	if !hasTabelog {
		t.Error("ForCountry(JP) should include Tabelog (canonical)")
	}
	if hasLeFooding {
		t.Error("ForCountry(JP) should NOT include Le Fooding (canonical for FR only)")
	}
}

func TestScore(t *testing.T) {
	tabelog := *BySlug("tabelog")

	// Tabelog scoring a Japanese food place at 10 walking minutes.
	gotJP := tabelog.Score(CountryJapan, 10, IntentFood)
	// Expected: trust 0.90 × (1 + 0.05 boost) × 1.0 intent × 1/(1 + 10/15)
	//         = 0.90 × 1.05 × 1.0 × 0.6 = 0.567
	if math.Abs(gotJP-0.567) > 0.01 {
		t.Errorf("Tabelog JP score: got %.3f, want ~0.567", gotJP)
	}

	// Tabelog scoring a Korean food place — no country boost.
	gotKR := tabelog.Score(CountryKorea, 10, IntentFood)
	// Expected: 0.90 × 1.0 × 1.0 × 0.6 = 0.54
	if math.Abs(gotKR-0.54) > 0.01 {
		t.Errorf("Tabelog KR score: got %.3f, want ~0.54", gotKR)
	}

	// Walking time decay: same source, longer walk = lower score.
	near := tabelog.Score(CountryJapan, 0, IntentFood)
	far := tabelog.Score(CountryJapan, 30, IntentFood)
	if near <= far {
		t.Errorf("near should outscore far: near=%.3f far=%.3f", near, far)
	}

	// Off-intent source: viewpoint at a Tabelog source halves intent match.
	off := tabelog.Score(CountryJapan, 10, IntentViewpoint)
	on := tabelog.Score(CountryJapan, 10, IntentFood)
	if off >= on {
		t.Errorf("off-intent should score lower: off=%.3f on=%.3f", off, on)
	}
}

func TestRegistryUniqueSlug(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Registry {
		if seen[s.Slug] {
			t.Errorf("duplicate slug %q in Registry", s.Slug)
		}
		seen[s.Slug] = true
		if s.Trust < 0 || s.Trust > 1 {
			t.Errorf("source %q has trust %v outside [0,1]", s.Slug, s.Trust)
		}
	}
}
