// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package scengine

import "testing"

// A source that tags a meta-analysis with nothing but the generic "review"
// pubtype used to cost the work eight tier points, because the cascade returned
// on the first authoritative label and never reached the heuristics. Measured on
// 10.3389/fnut.2022.1084455, a 55-RCT meta-analysis: tier 9 became tier 1.
func TestGenericReviewPubtypeYieldsToStrongerHeuristic(t *testing.T) {
	const (
		metaTitle = "The effects of green tea supplementation on cardiovascular " +
			"risk factors: A systematic review and meta-analysis"
		metaAbstract = "The current systematic review and meta-analysis study aimed " +
			"to establish the effects. Meta-analyses were carried out using a " +
			"random-effects model."
		plainTitle    = "Beneficial effects of green tea"
		plainAbstract = "An overview of what is known about green tea and health."
	)

	tests := []struct {
		name       string
		title      string
		abstract   string
		pubTypes   []string
		wantDesign Design
		wantMethod Method
	}{
		{
			name:       "generic review yields to a stronger heuristic",
			title:      metaTitle,
			abstract:   metaAbstract,
			pubTypes:   []string{"Review"},
			wantDesign: DesignMetaAnalysis,
			wantMethod: MethodHeuristic,
		},
		{
			name:       "generic review stands when the heuristics find nothing",
			title:      plainTitle,
			abstract:   plainAbstract,
			pubTypes:   []string{"Review"},
			wantDesign: DesignNarrativeReview,
			wantMethod: MethodPubMedPubType,
		},
		{
			name:       "a specific pubtype still wins outright",
			title:      plainTitle,
			abstract:   plainAbstract,
			pubTypes:   []string{"Meta-Analysis"},
			wantDesign: DesignMetaAnalysis,
			wantMethod: MethodPubMedPubType,
		},
		{
			name:       "a specific pubtype is not overridden by a weaker heuristic",
			title:      "A narrative review of green tea",
			abstract:   "This narrative review surveys the literature.",
			pubTypes:   []string{"Randomized Controlled Trial"},
			wantDesign: DesignRCT,
			wantMethod: MethodPubMedPubType,
		},
		{
			name:       "no pubtypes leaves the heuristics in charge",
			title:      metaTitle,
			abstract:   metaAbstract,
			pubTypes:   nil,
			wantDesign: DesignMetaAnalysis,
			wantMethod: MethodHeuristic,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyDesign(tc.title, tc.abstract, "review", tc.pubTypes)
			if got.Design != tc.wantDesign {
				t.Errorf("design = %q, want %q", got.Design, tc.wantDesign)
			}
			if got.Method != tc.wantMethod {
				t.Errorf("method = %q, want %q", got.Method, tc.wantMethod)
			}
			if want := TierWeight(tc.wantDesign); got.Tier != want {
				t.Errorf("tier = %v, want %v", got.Tier, want)
			}
		})
	}
}
