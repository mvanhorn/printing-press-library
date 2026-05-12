package refdata

import (
	"strings"
	"testing"
)

func TestCountSeeds(t *testing.T) {
	n, p := CountSeeds()
	if n < 20 {
		t.Errorf("expected at least 20 NAICS seeds, got %d", n)
	}
	if p < 20 {
		t.Errorf("expected at least 20 PSC seeds, got %d", p)
	}
}

func TestNAICSITCoreCodes(t *testing.T) {
	// Every federal-IT user's day-one codes must be in the seed
	mustHave := []string{"541511", "541512", "541513", "541519", "518210"}
	have := map[string]bool{}
	for _, c := range NAICSSeeds {
		have[c.Code] = true
	}
	for _, code := range mustHave {
		if !have[code] {
			t.Errorf("missing core IT NAICS seed: %s", code)
		}
	}
}

func TestPSCITCoreCodes(t *testing.T) {
	// Core D-series IT services codes must be in the seed
	mustHave := []string{"D301", "D302", "D307", "D310", "D316", "D330", "D399"}
	have := map[string]bool{}
	for _, c := range PSCSeeds {
		have[c.Code] = true
	}
	for _, code := range mustHave {
		if !have[code] {
			t.Errorf("missing core PSC seed: %s", code)
		}
	}
}

func TestNAICSSeedShape(t *testing.T) {
	for _, c := range NAICSSeeds {
		if c.Code == "" || c.Title == "" {
			t.Errorf("NAICS seed has empty required field: %+v", c)
		}
		if c.Depth < 0 || c.Depth > 6 {
			t.Errorf("NAICS depth out of range: %+v", c)
		}
	}
}

func TestPSCSeedShape(t *testing.T) {
	for _, c := range PSCSeeds {
		if c.Code == "" || c.Title == "" {
			t.Errorf("PSC seed has empty required field: %+v", c)
		}
		// PSC codes are alphanumeric (D-prefix, 70xx, etc.)
		if len(c.Code) < 1 {
			t.Errorf("PSC code too short: %+v", c)
		}
	}
}

func TestNoDuplicateNAICS(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range NAICSSeeds {
		if seen[c.Code] {
			t.Errorf("duplicate NAICS code in seeds: %s", c.Code)
		}
		seen[c.Code] = true
	}
}

func TestNoDuplicatePSC(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range PSCSeeds {
		if seen[c.Code] {
			t.Errorf("duplicate PSC code in seeds: %s", c.Code)
		}
		seen[c.Code] = true
	}
}

func TestNAICSITTitlesMatchVocabulary(t *testing.T) {
	// At least one seed should mention Computer Systems Design - that's the
	// canonical federal-IT NAICS title.
	for _, c := range NAICSSeeds {
		if c.Code == "541512" && strings.Contains(strings.ToLower(c.Title), "computer systems design") {
			return
		}
	}
	t.Errorf("NAICS 541512 should be titled 'Computer Systems Design Services'")
}

func TestPSCITTitlesMatchVocabulary(t *testing.T) {
	// D399 is "Other IT and Telecommunications" - a common catch-all
	for _, c := range PSCSeeds {
		if c.Code == "D399" {
			if !strings.Contains(strings.ToLower(c.Title), "other") {
				t.Errorf("PSC D399 title should describe Other IT, got %q", c.Title)
			}
			return
		}
	}
	t.Errorf("PSC D399 not in seeds")
}
