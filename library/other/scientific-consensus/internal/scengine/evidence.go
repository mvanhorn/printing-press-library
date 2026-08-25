// Package scengine implements the scientific-consensus analytic engines:
// study-design (evidence) classification, stance detection, and consensus
// scoring. It is pure logic with no network or store dependencies so it can be
// unit-tested in isolation and reused across commands.
package scengine

import (
	"regexp"
	"strings"
)

// Design is a study-design classification, ordered loosely from strongest to
// weakest evidence. The zero value is DesignUnknown.
type Design string

const (
	DesignUmbrellaReview   Design = "umbrella-review"
	DesignMetaAnalysis     Design = "meta-analysis"
	DesignSystematicReview Design = "systematic-review"
	DesignRCT              Design = "randomized-controlled-trial"
	DesignCohort           Design = "cohort-study"
	DesignCaseControl      Design = "case-control-study"
	DesignCrossSectional   Design = "cross-sectional-study"
	DesignCaseSeries       Design = "case-series"
	DesignCaseReport       Design = "case-report"
	DesignNarrativeReview  Design = "narrative-review"
	DesignUnknown          Design = "unclassified"
)

// PyramidOrder is the evidence pyramid from apex (strongest) to base (weakest).
// Render order for `evidence` output and tier weighting both derive from this.
var PyramidOrder = []Design{
	DesignUmbrellaReview,
	DesignMetaAnalysis,
	DesignSystematicReview,
	DesignRCT,
	DesignCohort,
	DesignCaseControl,
	DesignCrossSectional,
	DesignCaseSeries,
	DesignCaseReport,
	DesignNarrativeReview,
	DesignUnknown,
}

// tierWeight assigns each design an evidence weight used by the consensus
// engine. Higher = stronger evidence. Kept on a 1..10 scale.
var tierWeight = map[Design]float64{
	DesignUmbrellaReview:   10,
	DesignMetaAnalysis:     9,
	DesignSystematicReview: 8,
	DesignRCT:              7,
	DesignCohort:           5,
	DesignCaseControl:      4,
	DesignCrossSectional:   3,
	DesignCaseSeries:       2,
	DesignCaseReport:       1,
	DesignNarrativeReview:  1,
	DesignUnknown:          1,
}

// TierWeight returns the evidence weight for a design (>=1).
func TierWeight(d Design) float64 {
	if w, ok := tierWeight[d]; ok {
		return w
	}
	return 1
}

// TierRank returns the apex-to-base rank of a design (0 = strongest). Unknown
// designs rank last.
func TierRank(d Design) int {
	for i, o := range PyramidOrder {
		if o == d {
			return i
		}
	}
	return len(PyramidOrder)
}

// Method records how a classification was reached, so output never presents a
// heuristic guess as if it were authoritative.
type Method string

const (
	MethodPubMedPubType Method = "pubmed-pubtype" // authoritative MeSH publication type
	MethodPublication   Method = "publication-type"
	MethodHeuristic     Method = "title-abstract-heuristic"
	MethodOpenAlexType  Method = "openalex-type"
	MethodNone          Method = "none"
)

// Classification is the result of classifying one work.
type Classification struct {
	Design Design  `json:"design"`
	Tier   float64 `json:"tier_weight"`
	Method Method  `json:"method"`
}

// pubtypeMap maps authoritative publication-type labels (PubMed MeSH pubtype or
// Semantic Scholar publicationTypes) to a Design. Keys are lowercased.
var pubtypeMap = map[string]Design{
	"meta-analysis":               DesignMetaAnalysis,
	"systematic review":           DesignSystematicReview,
	"randomized controlled trial": DesignRCT,
	"controlled clinical trial":   DesignRCT,
	"clinical trial":              DesignRCT,
	"observational study":         DesignCohort,
	"case reports":                DesignCaseReport,
	"review":                      DesignNarrativeReview,
}

// Heuristic regexes, evaluated apex-first; first match wins.
var heuristicTiers = []struct {
	re     *regexp.Regexp
	design Design
}{
	{regexp.MustCompile(`(?i)\bumbrella review\b|\boverview of (systematic )?reviews\b`), DesignUmbrellaReview},
	{regexp.MustCompile(`(?i)\bmeta-?analy[sz]is\b|\bmeta-?analytic\b`), DesignMetaAnalysis},
	{regexp.MustCompile(`(?i)\bsystematic review\b|\bsystematic literature review\b`), DesignSystematicReview},
	{regexp.MustCompile(`(?i)\brandomi[sz]ed (controlled |clinical )?trial\b|\bdouble-?blind\b|\bplacebo-?controlled\b|\b\brct\b`), DesignRCT},
	{regexp.MustCompile(`(?i)\bcohort (study|studies)\b|\bprospective cohort\b|\bretrospective cohort\b|\blongitudinal (study|cohort)\b`), DesignCohort},
	{regexp.MustCompile(`(?i)\bcase[- ]control (study|studies)\b|\bnested case[- ]control\b`), DesignCaseControl},
	{regexp.MustCompile(`(?i)\bcross[- ]sectional\b|\bprevalence (study|survey)\b`), DesignCrossSectional},
	{regexp.MustCompile(`(?i)\bcase series\b`), DesignCaseSeries},
	{regexp.MustCompile(`(?i)\bcase report\b|\ba case of\b`), DesignCaseReport},
	{regexp.MustCompile(`(?i)\bnarrative review\b|\bliterature review\b|\bscoping review\b`), DesignNarrativeReview},
}

// ClassifyDesign determines a study design using the cascade documented in the
// research brief: authoritative publication types first (PubMed MeSH / Semantic
// Scholar), then title+abstract heuristics, then the coarse OpenAlex type, and
// finally Unknown. pubTypes may be nil.
func ClassifyDesign(title, abstract, openalexType string, pubTypes []string) Classification {
	// 1. Authoritative publication types (apex-first by tier rank).
	best := DesignUnknown
	bestRank := len(PyramidOrder)
	for _, pt := range pubTypes {
		if d, ok := pubtypeMap[strings.ToLower(strings.TrimSpace(pt))]; ok {
			if r := TierRank(d); r < bestRank {
				best, bestRank = d, r
			}
		}
	}
	// Every pubtype except the generic "review" names a specific design and is
	// trusted outright. "review" is the one label a source can apply to a work
	// whose own title and abstract call it a meta-analysis: measured on
	// 10.3389/fnut.2022.1084455, a 55-RCT meta-analysis tagged only "Review",
	// that costs the work eight tier points (9 -> 1). So when the generic label
	// is all that came back, the heuristics still run and the stronger of the
	// two answers wins.
	if best != DesignUnknown && best != DesignNarrativeReview {
		return Classification{Design: best, Tier: TierWeight(best), Method: MethodPubMedPubType}
	}

	// 2. Title + abstract heuristics.
	hay := strings.ToLower(title + ". " + abstract)
	for _, t := range heuristicTiers {
		if t.re.MatchString(hay) {
			if TierRank(t.design) < bestRank {
				return Classification{Design: t.design, Tier: TierWeight(t.design), Method: MethodHeuristic}
			}
			break
		}
	}
	// The generic pubtype stands when the heuristics found nothing stronger.
	if best != DesignUnknown {
		return Classification{Design: best, Tier: TierWeight(best), Method: MethodPubMedPubType}
	}

	// 3. Coarse OpenAlex type fallback.
	switch strings.ToLower(strings.TrimSpace(openalexType)) {
	case "review":
		return Classification{Design: DesignNarrativeReview, Tier: TierWeight(DesignNarrativeReview), Method: MethodOpenAlexType}
	case "article", "preprint":
		return Classification{Design: DesignUnknown, Tier: TierWeight(DesignUnknown), Method: MethodOpenAlexType}
	}

	return Classification{Design: DesignUnknown, Tier: TierWeight(DesignUnknown), Method: MethodNone}
}

// Pyramid counts classifications by design and returns counts in apex-to-base
// order alongside the apex design actually present.
type PyramidLevel struct {
	Design Design `json:"design"`
	Count  int    `json:"count"`
}

// Pyramid aggregates classifications into apex-to-base levels (zero-count
// levels included so the shape is stable).
func Pyramid(cs []Classification) []PyramidLevel {
	counts := map[Design]int{}
	for _, c := range cs {
		counts[c.Design]++
	}
	out := make([]PyramidLevel, 0, len(PyramidOrder))
	for _, d := range PyramidOrder {
		out = append(out, PyramidLevel{Design: d, Count: counts[d]})
	}
	return out
}

// ApexDesign returns the strongest design present in the set, or Unknown.
func ApexDesign(cs []Classification) Design {
	apex := DesignUnknown
	apexRank := len(PyramidOrder)
	for _, c := range cs {
		if r := TierRank(c.Design); r < apexRank {
			apex, apexRank = c.Design, r
		}
	}
	return apex
}
