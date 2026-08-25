package scengine

import "math"

// ScoredWork is the per-work input to the consensus engine.
type ScoredWork struct {
	Stance     Stance
	StanceConf float64
	Design     Design
	CitedBy    int
}

// Verdict summarizes the overall consensus direction.
type Verdict string

const (
	VerdictSupports     Verdict = "evidence-supports"
	VerdictRefutes      Verdict = "evidence-refutes"
	VerdictMixed        Verdict = "evidence-mixed"
	VerdictInconclusive Verdict = "inconclusive"
	VerdictInsufficient Verdict = "insufficient-evidence"
)

// EvidenceStrength is a coarse label for how strong the underlying evidence base is.
type EvidenceStrength string

const (
	StrengthHigh     EvidenceStrength = "high"
	StrengthModerate EvidenceStrength = "moderate"
	StrengthLow      EvidenceStrength = "low"
	StrengthVeryLow  EvidenceStrength = "very-low"
)

// ConsensusResult is the consensus engine's output.
type ConsensusResult struct {
	Verdict          Verdict          `json:"verdict"`
	ConsensusScore   float64          `json:"consensus_score"` // -1..1 (refute..support), tier+citation weighted
	Confidence       float64          `json:"confidence"`      // 0..1
	EvidenceStrength EvidenceStrength `json:"evidence_strength"`
	ApexDesign       Design           `json:"apex_design"`
	StudyCount       int              `json:"study_count"`
	Supporting       int              `json:"supporting"`
	Refuting         int              `json:"refuting"`
	Mixed            int              `json:"mixed"`
	Inconclusive     int              `json:"inconclusive"`
	TotalCitations   int              `json:"total_citations"`
	// NearUnanimous flags a result so one-sided that the absence of dissent is
	// itself suspicious — usually a sign that genuine debate was filtered out
	// upstream rather than that the science is settled. Presentation-only: it
	// never feeds back into Verdict, ConsensusScore, or Confidence.
	NearUnanimous bool `json:"near_unanimous"`
}

// Consensus aggregates scored works into a tier- and citation-weighted verdict.
func Consensus(works []ScoredWork) ConsensusResult {
	// ApexDesign is seeded explicitly: the zero value of Design is the empty
	// string, and the len==0 early return below skips the ApexDesign() call
	// that would otherwise fill it. Without the seed a zero-work result emits
	// "apex_design": "" — not a design at all, and not something a consumer can
	// map to a tier. Every non-empty corpus overwrites this below.
	res := ConsensusResult{
		StudyCount: len(works), Verdict: VerdictInsufficient,
		EvidenceStrength: StrengthVeryLow, ApexDesign: DesignUnknown,
	}
	if len(works) == 0 {
		return res
	}

	var weightedNet, weightedTotal float64
	designs := make([]Classification, 0, len(works))
	for _, w := range works {
		res.TotalCitations += w.CitedBy
		designs = append(designs, Classification{Design: w.Design})
		// weight = evidence tier * log-scaled citation mass * stance confidence
		w8 := TierWeight(w.Design) * (1 + math.Log1p(float64(max(w.CitedBy, 0)))) * clamp01(w.StanceConf, 0.3)
		switch w.Stance {
		case StanceSupporting:
			res.Supporting++
			weightedNet += w8
			weightedTotal += w8
		case StanceRefuting:
			res.Refuting++
			weightedNet -= w8
			weightedTotal += w8
		case StanceMixed:
			res.Mixed++
			weightedTotal += w8 * 0.5
		default:
			res.Inconclusive++
		}
	}

	res.ApexDesign = ApexDesign(designs)

	if weightedTotal > 0 {
		res.ConsensusScore = round2(weightedNet / weightedTotal)
	}

	directional := res.Supporting + res.Refuting
	res.Confidence = round2(confidence(res, directional))
	res.EvidenceStrength = strength(res.ApexDesign, res.StudyCount)
	res.Verdict = verdict(res, directional)
	res.NearUnanimous = nearUnanimous(res)
	return res
}

func verdict(r ConsensusResult, directional int) Verdict {
	if r.StudyCount < 3 || directional == 0 {
		if directional == 0 && r.Mixed > 0 {
			return VerdictMixed
		}
		return VerdictInsufficient
	}
	switch {
	case r.ConsensusScore >= 0.4:
		return VerdictSupports
	case r.ConsensusScore <= -0.4:
		return VerdictRefutes
	case r.Mixed >= directional:
		return VerdictMixed
	case r.ConsensusScore > -0.15 && r.ConsensusScore < 0.15:
		return VerdictInconclusive
	case r.ConsensusScore > 0:
		return VerdictSupports
	default:
		return VerdictRefutes
	}
}

// phase4ConfidenceEnabled controls the dispersion penalty on confidence.
// Always true in production; tests may toggle it.
var phase4ConfidenceEnabled = true

// dispersionWeight is how much of the confidence a fully divided corpus gives
// up. Package-level and tunable after measurement; at 0.35 a corpus that
// cancels out entirely keeps 65% of the confidence its volume and design would
// otherwise earn. It is deliberately not 1.0 — study count and apex design are
// still real signal even when the direction is contested.
var dispersionWeight = 0.35

// stanceDispersion measures how far the corpus is from speaking with one voice.
// It returns 0.0 when every work points the same way and 1.0 when supporting
// and refuting cancel out entirely. Unlike the agreement term it divides by the
// TOTAL work count, so mixed and inconclusive works count as evidence of
// uncertainty rather than being invisible.
func stanceDispersion(supporting, refuting, mixed, inconclusive int) float64 {
	total := supporting + refuting + mixed + inconclusive
	if total == 0 {
		return 0
	}
	net := supporting - refuting
	if net < 0 {
		net = -net
	}
	return 1 - float64(net)/float64(total)
}

func confidence(r ConsensusResult, directional int) float64 {
	if r.StudyCount == 0 {
		return 0
	}
	// More studies, stronger apex design, and stronger agreement raise confidence.
	volume := math.Min(1, float64(r.StudyCount)/25.0)
	apex := 1 - float64(TierRank(r.ApexDesign))/float64(len(PyramidOrder))
	var agreement float64
	if directional > 0 {
		agreement = math.Abs(float64(r.Supporting-r.Refuting)) / float64(directional)
	}
	conf := 0.45*volume + 0.30*apex + 0.25*agreement
	// Dispersion penalty. The agreement term above sees only directional works,
	// so a corpus of 1 supporting and 30 inconclusive scores a perfect 1.0 on
	// agreement — a corpus that knows almost nothing looks unanimous. The
	// penalty is applied multiplicatively, after the weighted sum, so it scales
	// the whole confidence rather than competing with any single term for
	// weight.
	if phase4ConfidenceEnabled {
		d := stanceDispersion(r.Supporting, r.Refuting, r.Mixed, r.Inconclusive)
		conf *= 1 - dispersionWeight*d
	}
	return math.Min(0.97, conf)
}

// strength labels the evidence base from design tier and volume alone.
// Directional agreement is deliberately not an input — that signal is
// carried by Confidence and Verdict.
func strength(apex Design, studies int) EvidenceStrength {
	rank := TierRank(apex)
	switch {
	case rank <= TierRank(DesignMetaAnalysis) && studies >= 5:
		return StrengthHigh
	case rank <= TierRank(DesignRCT) && studies >= 3:
		return StrengthModerate
	case rank <= TierRank(DesignCohort):
		return StrengthLow
	default:
		return StrengthVeryLow
	}
}

// nearUnanimousScore is the |ConsensusScore| at or above which a result with no
// dissent at all is treated as suspiciously perfect rather than merely strong.
const nearUnanimousScore = 0.98

// nearUnanimous reports whether a result is so one-sided that a real
// disagreement was probably filtered out before scoring. It requires ZERO
// refuting AND ZERO mixed studies: a single mixed study is enough evidence that
// dissent survived the pipeline, so the flag stays off. The check is symmetric
// (|score| >= 0.98) because a unanimous refutation is exactly as suspicious as
// a unanimous endorsement; the zero-dissent requirement means only one side can
// ever be populated anyway.
func nearUnanimous(r ConsensusResult) bool {
	if r.Refuting > 0 || r.Mixed > 0 {
		return false
	}
	if r.StudyCount == 0 {
		return false
	}
	return math.Abs(r.ConsensusScore) >= nearUnanimousScore
}

func clamp01(v, floor float64) float64 {
	if v < floor {
		return floor
	}
	if v > 1 {
		return 1
	}
	return v
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
