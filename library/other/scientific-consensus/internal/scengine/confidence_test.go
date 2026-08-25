package scengine

import "testing"

// --- confidence baseline ---------------------------------------------------
//
// confidence() had no test coverage before Phase 4. These pin the pre-Phase-4
// formula term by term so the dispersion penalty cannot silently move anything
// it is not meant to move.

const confEpsilon = 1e-6

func confNear(got, want float64) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d < confEpsilon
}

// TestConfidenceBaseline pins the PRE-Phase-4 formula, with the dispersion
// penalty switched off. These values were recorded against the code as it
// stood before the penalty existed, so the toggle is a genuine escape hatch:
// flipping it off must reproduce the old numbers exactly, not approximately.
func TestConfidenceBaseline(t *testing.T) {
	defer func(prev bool) { phase4ConfidenceEnabled = prev }(phase4ConfidenceEnabled)
	phase4ConfidenceEnabled = false

	tests := []struct {
		name string
		r    ConsensusResult
		want float64
	}{
		{
			// volume 10/25=0.4, apex meta-analysis rank1 -> 1-1/11, agreement 1
			name: "unanimous small meta-analysis corpus",
			r:    ConsensusResult{StudyCount: 10, ApexDesign: DesignMetaAnalysis, Supporting: 10},
			want: 0.45*0.4 + 0.30*(1-1.0/11.0) + 0.25*1,
		},
		{
			// volume capped at 1, apex RCT rank3, agreement 0 (perfect split)
			name: "evenly split large rct corpus",
			r:    ConsensusResult{StudyCount: 30, ApexDesign: DesignRCT, Supporting: 15, Refuting: 15},
			want: 0.45*1 + 0.30*(1-3.0/11.0) + 0.25*0,
		},
		{
			// volume 3/25, apex unknown rank10, agreement |2-1|/3
			name: "thin corpus unknown design",
			r:    ConsensusResult{StudyCount: 3, ApexDesign: DesignUnknown, Supporting: 2, Refuting: 1},
			want: 0.45*(3.0/25.0) + 0.30*(1-10.0/11.0) + 0.25*(1.0/3.0),
		},
		{
			// every term maxed -> hits the 0.97 ceiling
			name: "ceiling",
			r:    ConsensusResult{StudyCount: 40, ApexDesign: DesignUmbrellaReview, Supporting: 40},
			want: 0.97,
		},
		{
			name: "empty corpus",
			r:    ConsensusResult{},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directional := tt.r.Supporting + tt.r.Refuting
			got := confidence(tt.r, directional)
			if !confNear(got, tt.want) {
				t.Errorf("confidence = %.9f, want %.9f", got, tt.want)
			}
			t.Logf("BASELINE %s = %.6f", tt.name, got)
		})
	}
}

func TestStanceDispersion(t *testing.T) {
	tests := []struct {
		name                                      string
		supporting, refuting, mixed, inconclusive int
		want                                      float64
	}{
		{"unanimous supporting", 10, 0, 0, 0, 0.0},
		{"unanimous refuting", 0, 10, 0, 0, 0.0},
		{"even split", 5, 5, 0, 0, 1.0},
		{"lopsided but contested", 8, 2, 0, 0, 0.4},
		// The sharpest failure of the agreement term: it sees only directional
		// works, so this corpus scores a perfect 1.0 on agreement while knowing
		// essentially nothing. Dispersion must see it for what it is.
		{"one directional work among thirty unknowns", 1, 0, 0, 30, 1 - 1.0/31.0},
		{"mixed works count as dissent", 6, 0, 4, 0, 0.4},
		{"empty corpus is not dispersed", 0, 0, 0, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stanceDispersion(tt.supporting, tt.refuting, tt.mixed, tt.inconclusive)
			if !confNear(got, tt.want) {
				t.Errorf("stanceDispersion(%d,%d,%d,%d) = %.6f, want %.6f",
					tt.supporting, tt.refuting, tt.mixed, tt.inconclusive, got, tt.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("dispersion out of range: %v", got)
			}
		})
	}
}

// TestStanceDispersionMonotonic checks the property the value is meant to have:
// holding the corpus size fixed, moving works away from the dominant stance
// never lowers dispersion.
func TestStanceDispersionMonotonic(t *testing.T) {
	prev := -1.0
	for refuting := 0; refuting <= 10; refuting++ {
		got := stanceDispersion(20-refuting, refuting, 0, 0)
		if got < prev {
			t.Fatalf("dispersion dropped as dissent grew: refuting=%d gave %.4f after %.4f",
				refuting, got, prev)
		}
		prev = got
	}
}

// TestConfidencePhase4 pins the post-penalty values for the same inputs the
// baseline test uses, so the delta the penalty introduces is explicit.
func TestConfidencePhase4(t *testing.T) {
	if !phase4ConfidenceEnabled {
		t.Fatal("phase4ConfidenceEnabled must default to true in production")
	}
	tests := []struct {
		name string
		r    ConsensusResult
		want float64
	}{
		{
			// Unanimous: dispersion 0, so the penalty is a no-op and the
			// baseline value survives untouched.
			name: "unanimous corpus is not penalized",
			r:    ConsensusResult{StudyCount: 10, ApexDesign: DesignMetaAnalysis, Supporting: 10},
			want: 0.45*0.4 + 0.30*(1-1.0/11.0) + 0.25*1,
		},
		{
			// Perfectly split: dispersion 1, so the full weight applies.
			name: "evenly split corpus loses the full weight",
			r:    ConsensusResult{StudyCount: 30, ApexDesign: DesignRCT, Supporting: 15, Refuting: 15},
			want: (0.45*1 + 0.30*(1-3.0/11.0)) * (1 - dispersionWeight),
		},
		{
			name: "thin contested corpus",
			r:    ConsensusResult{StudyCount: 3, ApexDesign: DesignUnknown, Supporting: 2, Refuting: 1},
			want: (0.45*(3.0/25.0) + 0.30*(1-10.0/11.0) + 0.25*(1.0/3.0)) *
				(1 - dispersionWeight*(1-1.0/3.0)),
		},
		{
			name: "ceiling still applies after the penalty",
			r:    ConsensusResult{StudyCount: 40, ApexDesign: DesignUmbrellaReview, Supporting: 40},
			want: 0.97,
		},
		{
			name: "empty corpus",
			r:    ConsensusResult{},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := confidence(tt.r, tt.r.Supporting+tt.r.Refuting)
			if !confNear(got, tt.want) {
				t.Errorf("confidence = %.9f, want %.9f", got, tt.want)
			}
		})
	}
}

// TestConfidenceSmallUnanimousBeatsLargeDivided is the behavior Phase 4 exists
// for: study count is not evidence quality. Ten works that agree must outrank
// a hundred that cancel out.
func TestConfidenceSmallUnanimousBeatsLargeDivided(t *testing.T) {
	small := ConsensusResult{StudyCount: 10, ApexDesign: DesignRCT, Supporting: 10}
	large := ConsensusResult{StudyCount: 100, ApexDesign: DesignRCT, Supporting: 50, Refuting: 50}

	smallConf := confidence(small, small.Supporting+small.Refuting)
	largeConf := confidence(large, large.Supporting+large.Refuting)
	if smallConf <= largeConf {
		t.Errorf("small unanimous corpus (%.4f) should outrank large divided corpus (%.4f)",
			smallConf, largeConf)
	}

	// Without the penalty the ordering is the wrong way round — that inversion
	// is the defect Phase 4 corrects, and pinning it here documents why the
	// penalty cannot simply be deleted.
	defer func(prev bool) { phase4ConfidenceEnabled = prev }(phase4ConfidenceEnabled)
	phase4ConfidenceEnabled = false
	if confidence(small, 10) >= confidence(large, 100) {
		t.Errorf("pre-Phase-4 formula was expected to rank the large divided corpus higher; "+
			"small=%.4f large=%.4f", confidence(small, 10), confidence(large, 100))
	}
}

// TestConfidenceInconclusiveMassIsPenalized covers the case the agreement term
// is blind to: one directional work surrounded by works that concluded nothing.
func TestConfidenceInconclusiveMassIsPenalized(t *testing.T) {
	r := ConsensusResult{StudyCount: 31, ApexDesign: DesignCohort, Supporting: 1, Inconclusive: 30}

	withPenalty := confidence(r, 1)

	defer func(prev bool) { phase4ConfidenceEnabled = prev }(phase4ConfidenceEnabled)
	phase4ConfidenceEnabled = false
	withoutPenalty := confidence(r, 1)

	if withPenalty >= withoutPenalty {
		t.Errorf("a corpus of 1 supporting + 30 inconclusive must lose confidence: "+
			"with=%.4f without=%.4f", withPenalty, withoutPenalty)
	}
}
