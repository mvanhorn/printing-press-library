// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"math"
	"testing"
)

func TestRelevance(t *testing.T) {
	tests := []struct {
		name      string
		seed      string
		candidate string
		want      float64
	}{
		// The real rows the seeded endpoint returns for "dad shirt".
		{"exact", "dad shirt", "dad shirt", 1},
		{"plural stems to same", "dad shirt", "dad shirts", 1},
		{"possessive plural", "dad shirt", "dads shirt", 1},
		{"reordered", "dad shirt", "shirts dad", 1},
		{"extra words still fully on target", "dad shirt", "Funny Dad Shirt, Just Resting My Eyes", 1},
		{"half match", "dad shirt", "dad hat", 0.5},

		// The rows the DEFAULT FEED returned for "dad shirt" — the #1492 bug.
		// These must score at or near zero, which is what makes them fail the
		// evidence threshold instead of being counted as support.
		{"default-feed junk keyword", "dad shirt", "gifte fully", 0},
		{"default-feed junk keyword 2", "dad shirt", "gifttings", 0},
		{"default-feed unrelated product", "dad shirt", "Cute Flowers Lady Design Mug - Ceramic Coffee Cup", 0},
		{"default-feed unrelated product 2", "dad shirt", "Down to Earth DVD", 0},
		{"dad hat is not a dad shirt", "dad shirt", "Vintage Dad Hat, Embroidered Cap", 0.5},

		{"empty seed", "", "dad shirt", 0},
		{"empty candidate", "dad shirt", "", 0},
		{"stopwords ignored in seed", "shirt for dad", "dad shirt", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Relevance(tt.seed, tt.candidate)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("Relevance(%q, %q) = %v, want %v", tt.seed, tt.candidate, got, tt.want)
			}
		})
	}
}

// The #1492 headline regression: the default feed must not clear the relevance
// bar, and the seeded results must.
func TestRelevance_DefaultFeedFailsSeededPasses(t *testing.T) {
	seed := "dad shirt"

	seeded := []string{
		"dadness shirt", "dadfully shirt", "dad shirt dad", "dad dad shirts",
		"shirts dad", "dadly shirt", "dad-shirts", "dads shirt",
	}
	for _, kw := range seeded {
		if got := Relevance(seed, kw); got < DefaultMinRelevance {
			t.Errorf("seeded keyword %q scored %v, below the %v evidence bar", kw, got, DefaultMinRelevance)
		}
	}

	defaultFeed := []string{
		"gifte fully", "gift gifte gifting", "gifttings", "giftedly", "????gift",
	}
	for _, kw := range defaultFeed {
		if got := Relevance(seed, kw); got >= DefaultMinRelevance {
			t.Errorf("default-feed junk %q scored %v, at/above the %v bar — it would count as evidence", kw, got, DefaultMinRelevance)
		}
	}
}

func TestProductTypeMatch(t *testing.T) {
	physicalTee := ProductRow{Title: "Funny Dad Shirt, Comfort Colors Tee", ListingType: "physical", Tags: []string{"dad shirt", "funny tee"}}
	svgCutFile := ProductRow{Title: "Dad SVG Bundle, Cricut Cut File", ListingType: "download", Tags: []string{"svg", "cricut", "digital download"}}
	// Upstream mislabels this as physical, but the tags say otherwise.
	mislabeledDigital := ProductRow{Title: "Dad PNG Sublimation Design", ListingType: "physical", Tags: []string{"png", "sublimation design"}}
	physicalMug := ProductRow{Title: "Best Dad Ever Ceramic Mug", ListingType: "physical", Tags: []string{"mug"}}

	tests := []struct {
		name          string
		p             ProductRow
		productType   string
		excludeSVGPNG bool
		want          bool
	}{
		{"physical tee is physical", physicalTee, "physical", false, true},
		{"physical tee is apparel", physicalTee, "apparel", false, true},
		{"mug is physical but not apparel", physicalMug, "apparel", false, false},
		{"mug is physical", physicalMug, "physical", false, true},
		{"svg is digital", svgCutFile, "digital", false, true},
		{"svg is not physical", svgCutFile, "physical", false, false},
		{"svg excluded when excludeSVGPNG", svgCutFile, "", true, false},
		{"mislabeled digital caught by tags", mislabeledDigital, "physical", false, false},
		{"mislabeled digital excluded by flag", mislabeledDigital, "apparel", true, false},
		{"empty type accepts anything", physicalMug, "", false, true},
		{"unknown type rejects rather than passing all", physicalTee, "nonsense", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProductTypeMatch(tt.p, tt.productType, tt.excludeSVGPNG); got != tt.want {
				t.Errorf("ProductTypeMatch(%q, type=%q, excl=%v) = %v, want %v", tt.p.Title, tt.productType, tt.excludeSVGPNG, got, tt.want)
			}
		})
	}
}

// Confidence is the #1492 contract: it must never look confident without evidence.
func TestConfidence(t *testing.T) {
	tests := []struct {
		name string
		ev   EvidenceSet
		want float64
	}{
		{
			// THE #1492 BUG: the published CLI reported confidence 1.0 with
			// keyword_research: 0. That must now be impossible.
			name: "zero keyword evidence is capped at 0.5",
			ev:   EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 0, ProductsReturned: 20, ProductsRelevant: 20},
			want: 0.5,
		},
		{
			name: "no relevant evidence at all is zero",
			ev:   EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 0, ProductsReturned: 20, ProductsRelevant: 0},
			want: 0,
		},
		{
			name: "nothing returned is zero",
			ev:   EvidenceSet{},
			want: 0,
		},
		{
			name: "fully relevant, ample evidence",
			ev:   EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 20, ProductsReturned: 20, ProductsRelevant: 20},
			want: 1,
		},
		{
			name: "thin evidence lowers confidence even at perfect precision",
			ev:   EvidenceSet{KeywordsReturned: 1, KeywordsRelevant: 1, ProductsReturned: 1, ProductsRelevant: 1},
			want: 0.2, // precision 1.0 * coverage 2/10
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Confidence(tt.ev)
			if math.Abs(got-tt.want) > 0.011 {
				t.Errorf("Confidence(%+v) = %v, want %v", tt.ev, got, tt.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("Confidence out of range: %v", got)
			}
		})
	}
}

func TestOpportunityScore_ZeroWithoutEvidence(t *testing.T) {
	// An unknown niche scores zero, never "unmeasured but promising".
	if got := OpportunityScore(5000, 100, EvidenceSet{}); got != 0 {
		t.Errorf("OpportunityScore with no evidence = %v, want 0", got)
	}
	// Zero demand cannot be an opportunity however uncrowded.
	ev := EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 20}
	if got := OpportunityScore(0, 0, ev); got != 0 {
		t.Errorf("OpportunityScore with zero demand = %v, want 0", got)
	}
}

func TestOpportunityScore_UncrowdedBeatsCrowded(t *testing.T) {
	ev := EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 20, ProductsReturned: 20, ProductsRelevant: 20}
	uncrowded := OpportunityScore(2747, 167, ev)  // real: "dadfully shirt"
	crowded := OpportunityScore(2747, 805139, ev) // real: "dadness shirt"
	if uncrowded <= crowded {
		t.Errorf("uncrowded niche (%v) should outscore crowded one (%v) at equal demand", uncrowded, crowded)
	}
}

func TestLowCompetition(t *testing.T) {
	ample := EvidenceSet{KeywordsReturned: 20, KeywordsRelevant: 20, ProductsReturned: 20, ProductsRelevant: 16}

	// Real numbers from the live API for "dadfully shirt": 2747 vol / 167 comp.
	if ok, why := LowCompetition(2747, 167, ample); !ok {
		t.Errorf("2747 demand vs 167 competition should qualify as low-competition, got refusal: %s", why)
	}
	// Real numbers for "dad shirt" itself: 2.6K vol / 698K comp — crowded.
	if ok, _ := LowCompetition(2747, 698000, ample); ok {
		t.Error("2747 demand vs 698k competition must not be labeled low-competition")
	}
	// No evidence: the label is never applied on faith.
	if ok, why := LowCompetition(2747, 167, EvidenceSet{}); ok || why == "" {
		t.Errorf("low-competition must be refused (with a reason) when there is no evidence; got ok=%v why=%q", ok, why)
	}
	// Demand floor: nobody searching means no opportunity.
	if ok, _ := LowCompetition(5, 0, ample); ok {
		t.Error("a niche below the demand floor must not be labeled low-competition")
	}
}

func TestComputePriceBand(t *testing.T) {
	band := ComputePriceBand([]ProductRow{
		{Price: 30}, {Price: 10}, {Price: 20}, {Price: 0}, // 0 is "no price", excluded
	})
	if band.Count != 3 {
		t.Errorf("Count = %d, want 3 (zero-priced row excluded)", band.Count)
	}
	if band.Min != 10 || band.Median != 20 || band.Max != 30 {
		t.Errorf("band = %+v, want min 10 median 20 max 30", band)
	}
	empty := ComputePriceBand(nil)
	if empty.Count != 0 || empty.Median != 0 {
		t.Errorf("empty band = %+v, want zero value", empty)
	}
}

func TestSaturation(t *testing.T) {
	if got := Saturation(0, 100); !math.IsInf(got, 1) {
		t.Errorf("Saturation with zero demand = %v, want +Inf", got)
	}
	if got := Saturation(0, 0); got != 0 {
		t.Errorf("Saturation(0,0) = %v, want 0", got)
	}
	if got := Saturation(100, 500); got != 5 {
		t.Errorf("Saturation(100,500) = %v, want 5", got)
	}
}
