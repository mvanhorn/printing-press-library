package scengine

import "testing"

func TestClassifyDesign_PubTypeAuthoritative(t *testing.T) {
	tests := []struct {
		name     string
		pubTypes []string
		want     Design
		method   Method
	}{
		{"meta-analysis pubtype", []string{"Journal Article", "Meta-Analysis"}, DesignMetaAnalysis, MethodPubMedPubType},
		{"systematic review pubtype", []string{"Systematic Review"}, DesignSystematicReview, MethodPubMedPubType},
		{"rct pubtype", []string{"Randomized Controlled Trial"}, DesignRCT, MethodPubMedPubType},
		{"apex wins over review", []string{"Review", "Meta-Analysis"}, DesignMetaAnalysis, MethodPubMedPubType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyDesign("Some title", "Some abstract", "article", tt.pubTypes)
			if got.Design != tt.want {
				t.Errorf("design = %q, want %q", got.Design, tt.want)
			}
			if got.Method != tt.method {
				t.Errorf("method = %q, want %q", got.Method, tt.method)
			}
		})
	}
}

func TestClassifyDesign_Heuristic(t *testing.T) {
	tests := []struct {
		title string
		want  Design
	}{
		{"A systematic review and meta-analysis of vitamin D", DesignMetaAnalysis},
		{"A systematic review of statin therapy", DesignSystematicReview},
		{"A double-blind, placebo-controlled randomized trial of zinc", DesignRCT},
		{"A prospective cohort study of diet and mortality", DesignCohort},
		{"A nested case-control study of smoking", DesignCaseControl},
		{"A cross-sectional survey of sleep habits", DesignCrossSectional},
		{"Case report: an unusual presentation of lupus", DesignCaseReport},
		{"Umbrella review of exercise interventions", DesignUmbrellaReview},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := ClassifyDesign(tt.title, "", "article", nil)
			if got.Design != tt.want {
				t.Errorf("design = %q, want %q", got.Design, tt.want)
			}
			if got.Method != MethodHeuristic {
				t.Errorf("method = %q, want heuristic", got.Method)
			}
		})
	}
}

func TestClassifyDesign_Fallback(t *testing.T) {
	got := ClassifyDesign("An ordinary article", "no design cues here", "article", nil)
	if got.Design != DesignUnknown {
		t.Errorf("design = %q, want unknown", got.Design)
	}
	rev := ClassifyDesign("Thoughts on a topic", "", "review", nil)
	if rev.Design != DesignNarrativeReview || rev.Method != MethodOpenAlexType {
		t.Errorf("review fallback = %q/%q", rev.Design, rev.Method)
	}
}

func TestClassifyStance(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		abstract string
		want     Stance
	}{
		{"supporting", "Vitamin D and infection", "Supplementation significantly reduced the risk of respiratory infection and improved outcomes.", StanceSupporting},
		{"refuting", "Vitamin D trial", "There was no significant association and supplementation did not reduce infection rates.", StanceRefuting},
		{"harm", "Beta-carotene trial", "Supplementation was associated with a higher risk of lung cancer and adverse outcomes.", StanceRefuting},
		{"inconclusive", "A descriptive paper", "We describe the prevalence of a condition across regions.", StanceInconclusive},
		// Regression: harm-context "increas*" phrasing must not count as a
		// positive/support cue (RE2 has no lookahead; excluded per-match).
		{"increased risk is harm", "Processed meat and cancer", "Consumption was associated with increased risk of colorectal cancer.", StanceRefuting},
		{"increased the risk is harm", "Smoking study", "Smoking increased the risk of stroke in all age groups.", StanceRefuting},
		{"increased mortality is harm", "Drug X trial", "Treatment with drug X increased mortality compared with placebo.", StanceRefuting},
		{"increased incidence is harm", "Screening cohort", "Exposure increased incidence of adverse events.", StanceRefuting},
		// True positive-increase claims must still count as support.
		{"increased survival supports", "Exercise program", "The program significantly increased survival rates and improved quality of life.", StanceSupporting},
		{"increase in remission supports", "Therapy trial", "Therapy led to an increase in remission and improved adherence.", StanceSupporting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := ClassifyStance(tt.title, tt.abstract, "")
			if got != tt.want {
				t.Errorf("stance = %q, want %q (conf %.2f)", got, tt.want, conf)
			}
			if conf < 0 || conf > 1 {
				t.Errorf("confidence out of range: %v", conf)
			}
		})
	}
}

// TestClassifyStance_ClaimAware pins the claim-aware polarity behavior (FIX A).
// Today ClassifyStance ignores the claim and derives stance only from whether
// the paper reports a beneficial finding, so a HARM-asserting claim inverts:
// a paper reporting the harm should SUPPORT the claim, and a paper reporting
// benefit / no-effect should REFUTE it. BENEFIT-asserting and ambiguous claims
// must keep today's behavior exactly (no regression).
//
// These cases FAIL against current code (the two harm cases classify as
// inconclusive today because the base cue vocabulary is risk/mortality/incidence
// and never recognizes "weight gain"; see repro in the plan).
func TestClassifyStance_ClaimAware(t *testing.T) {
	tests := []struct {
		name     string
		claim    string
		title    string
		abstract string
		want     Stance
	}{
		{
			name:     "harm claim, paper shows less of the harm -> refuting",
			claim:    "artificial sweeteners cause weight gain",
			title:    "Non-nutritive sweeteners and body weight: a randomized trial",
			abstract: "The sweetener group showed less weight gain than the sugar group over 12 weeks.",
			want:     StanceRefuting,
		},
		{
			name:     "harm claim, paper shows more of the harm -> supporting",
			claim:    "artificial sweeteners cause weight gain",
			title:    "Sweetener consumption and adiposity: a prospective cohort",
			abstract: "Sweetener consumption was associated with greater weight gain over 10 years of follow-up.",
			want:     StanceSupporting,
		},
		{
			name:     "benefit claim, paper reports the benefit -> supporting (no regression)",
			claim:    "coffee improves alertness",
			title:    "Caffeine and cognitive performance",
			abstract: "Caffeine significantly improved alertness and vigilance in a double-blind trial.",
			want:     StanceSupporting,
		},
		{
			name:     "ambiguous claim -> falls back to today's claim-agnostic behavior",
			claim:    "the relationship between coffee and alertness",
			title:    "Caffeine and cognitive performance",
			abstract: "Caffeine significantly improved alertness and vigilance in a double-blind trial.",
			// Expectation is defined relative to today's behavior below.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := ClassifyStance(tt.title, tt.abstract, tt.claim)
			want := tt.want
			if tt.name == "ambiguous claim -> falls back to today's claim-agnostic behavior" {
				// Ambiguous polarity must be indistinguishable from the
				// claim-agnostic baseline (empty claim) — no crash, no drift.
				want, _ = ClassifyStance(tt.title, tt.abstract, "")
			}
			if got != want {
				t.Errorf("stance = %q, want %q (conf %.2f)", got, want, conf)
			}
			if conf < 0 || conf > 1 {
				t.Errorf("confidence out of range: %v", conf)
			}
		})
	}
}

func TestConsensus(t *testing.T) {
	works := []ScoredWork{
		{Stance: StanceSupporting, StanceConf: 0.8, Design: DesignMetaAnalysis, CitedBy: 500},
		{Stance: StanceSupporting, StanceConf: 0.7, Design: DesignRCT, CitedBy: 200},
		{Stance: StanceSupporting, StanceConf: 0.7, Design: DesignSystematicReview, CitedBy: 150},
		{Stance: StanceRefuting, StanceConf: 0.6, Design: DesignCaseReport, CitedBy: 5},
	}
	r := Consensus(works)
	if r.Verdict != VerdictSupports {
		t.Errorf("verdict = %q, want supports", r.Verdict)
	}
	if r.ConsensusScore <= 0 {
		t.Errorf("score = %v, want > 0", r.ConsensusScore)
	}
	if r.StudyCount != 4 || r.Supporting != 3 || r.Refuting != 1 {
		t.Errorf("counts wrong: %+v", r)
	}
	if r.ApexDesign != DesignMetaAnalysis {
		t.Errorf("apex = %q, want meta-analysis", r.ApexDesign)
	}
	if r.TotalCitations != 855 {
		t.Errorf("total citations = %d, want 855", r.TotalCitations)
	}
}

func TestConsensus_Insufficient(t *testing.T) {
	r := Consensus([]ScoredWork{{Stance: StanceSupporting, StanceConf: 0.8, Design: DesignRCT, CitedBy: 10}})
	if r.Verdict != VerdictInsufficient {
		t.Errorf("verdict = %q, want insufficient for n<3", r.Verdict)
	}
	empty := Consensus(nil)
	if empty.Verdict != VerdictInsufficient || empty.StudyCount != 0 {
		t.Errorf("empty consensus wrong: %+v", empty)
	}
}

// strength labels the evidence base from apex design + volume only.
func TestStrength(t *testing.T) {
	tests := []struct {
		name    string
		apex    Design
		studies int
		want    EvidenceStrength
	}{
		{"meta-analysis with volume", DesignMetaAnalysis, 5, StrengthHigh},
		{"meta-analysis thin volume", DesignMetaAnalysis, 4, StrengthModerate},
		{"rct with volume", DesignRCT, 3, StrengthModerate},
		{"rct thin volume", DesignRCT, 2, StrengthLow},
		{"cohort apex", DesignCohort, 10, StrengthLow},
		{"case report apex", DesignCaseReport, 10, StrengthVeryLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strength(tt.apex, tt.studies); got != tt.want {
				t.Errorf("strength(%q, %d) = %q, want %q", tt.apex, tt.studies, got, tt.want)
			}
		})
	}
}

func TestPyramidAndApex(t *testing.T) {
	cs := []Classification{
		{Design: DesignRCT}, {Design: DesignRCT}, {Design: DesignCaseReport}, {Design: DesignMetaAnalysis},
	}
	levels := Pyramid(cs)
	if len(levels) != len(PyramidOrder) {
		t.Fatalf("pyramid levels = %d, want %d", len(levels), len(PyramidOrder))
	}
	byDesign := map[Design]int{}
	for _, l := range levels {
		byDesign[l.Design] = l.Count
	}
	if byDesign[DesignRCT] != 2 || byDesign[DesignMetaAnalysis] != 1 || byDesign[DesignCaseReport] != 1 {
		t.Errorf("pyramid counts wrong: %v", byDesign)
	}
	if ApexDesign(cs) != DesignMetaAnalysis {
		t.Errorf("apex = %q, want meta-analysis", ApexDesign(cs))
	}
}

func TestReconstructAbstract(t *testing.T) {
	inv := map[string][]int{"Vitamin": {0}, "D": {1}, "reduces": {2}, "risk": {3}}
	got := ReconstructAbstract(inv)
	want := "Vitamin D reduces risk"
	if got != want {
		t.Errorf("abstract = %q, want %q", got, want)
	}
	if ReconstructAbstract(nil) != "" {
		t.Errorf("nil index should yield empty string")
	}
}
