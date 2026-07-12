// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Relevance thresholds. These are documented constants rather than magic
// numbers because issue #1492's acceptance criteria are stated in terms of
// them: a "low competition" label requires documented thresholds, and
// confidence must be tied to evidence coverage.
const (
	// DefaultMinRelevance is the default --min-relevance floor. Rows below it
	// are still returned (annotate-only), they just do not count as evidence.
	DefaultMinRelevance = 0.5

	// MaxConfidenceWithoutKeywordEvidence caps confidence when zero keyword
	// rows were relevant. #1492: "Confidence is never above 0.5 when keyword
	// evidence is zero."
	MaxConfidenceWithoutKeywordEvidence = 0.5

	// LowCompetitionSaturation is the documented saturation ceiling below which
	// a niche may be labeled low-competition. Saturation is competing listings
	// per unit of monthly search volume.
	LowCompetitionSaturation = 50.0

	// MinDemandForLowCompetitionClaim is the documented demand floor. A niche
	// nobody searches for is not an opportunity, however uncrowded.
	MinDemandForLowCompetitionClaim = 100
)

var tokenSplit = regexp.MustCompile(`[^a-z0-9]+`)

// englishStopwords are dropped from seed tokens so that a seed like
// "shirt for dad" does not treat "for" as a term a candidate must contain.
var englishStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "for": true, "of": true, "and": true,
	"or": true, "to": true, "in": true, "on": true, "with": true, "by": true,
	"my": true, "your": true, "is": true, "are": true,
}

// Tokenize lowercases, splits on non-alphanumerics, drops stopwords, and
// stems trivially (trailing "s"). Trivial stemming matters: EverBee returns
// "dad shirts" and "dads shirt" as distinct keywords for the seed "dad shirt",
// and a caller would rightly consider both relevant.
func Tokenize(s string) []string {
	raw := tokenSplit.Split(strings.ToLower(strings.TrimSpace(s)), -1)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if t == "" || englishStopwords[t] {
			continue
		}
		out = append(out, stem(t))
	}
	return out
}

// stem strips a single trailing plural "s" from tokens long enough for it to
// be a plural rather than part of the word ("as", "is" are left alone).
func stem(t string) string {
	if len(t) > 3 && strings.HasSuffix(t, "s") && !strings.HasSuffix(t, "ss") {
		return strings.TrimSuffix(t, "s")
	}
	return t
}

// Relevance scores how much of the seed's meaning a candidate string carries,
// as the fraction of the seed's tokens present in the candidate. It is
// deliberately mechanical (token overlap, not NLP): the result must be
// explainable to a user asking "why did you call this relevant?".
//
// Returns 0 for an empty seed or empty candidate. A candidate containing every
// seed token scores 1.0 regardless of how many extra words it adds, because
// "Funny Dad Shirt, Just Resting My Eyes" is fully on-target for "dad shirt".
func Relevance(seed, candidate string) float64 {
	seedTokens := Tokenize(seed)
	if len(seedTokens) == 0 {
		return 0
	}
	candTokens := Tokenize(candidate)
	if len(candTokens) == 0 {
		return 0
	}
	have := make(map[string]bool, len(candTokens))
	for _, t := range candTokens {
		have[t] = true
	}
	matched := 0
	for _, t := range seedTokens {
		if have[t] {
			matched++
		}
	}
	return float64(matched) / float64(len(seedTokens))
}

// digitalTagPattern matches tags and titles that mark a listing as a digital
// download rather than a physical good. #1492 F9 requires SVG/PNG exclusion:
// an apparel researcher must not have cut-file listings pollute their evidence.
var digitalTagPattern = regexp.MustCompile(`(?i)\b(svg|png|dxf|eps|jpg|jpeg|pdf|cricut|silhouette|cut file|cutfile|digital download|instant download|printable|clipart|sublimation design)\b`)

// apparelPattern matches listings that are wearable goods.
var apparelPattern = regexp.MustCompile(`(?i)\b(shirt|t-shirt|tshirt|tee|sweatshirt|hoodie|hoody|crewneck|tank top|apparel|jersey|cap|hat|beanie|sock|sweater)\b`)

// ProductTypeMatch reports whether a product satisfies a requested product-type
// constraint. Recognized types: "physical", "digital", "apparel", and "" (any).
//
// EverBee's own listing_type field is the primary signal; tags and title text
// are the fallback, because a physical-typed listing whose tags scream "SVG
// cricut cut file" is a digital product mislabeled upstream.
func ProductTypeMatch(p ProductRow, productType string, excludeSVGPNG bool) bool {
	hay := p.Title + " " + strings.Join(p.Tags, " ")
	looksDigital := digitalTagPattern.MatchString(hay) ||
		strings.EqualFold(p.ListingType, "download") ||
		strings.EqualFold(p.ListingType, "digital")

	if excludeSVGPNG && looksDigital {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(productType)) {
	case "", "any":
		return true
	case "physical":
		return !looksDigital
	case "digital":
		return looksDigital
	case "apparel":
		return !looksDigital && apparelPattern.MatchString(hay)
	default:
		// An unrecognized constraint must not silently pass everything.
		return false
	}
}

// Saturation is competing listings per unit of monthly search demand. Higher
// means more crowded. Demand of zero yields +Inf, which callers surface as
// "no demand" rather than as an attractive score.
func Saturation(demand, competition int) float64 {
	if demand <= 0 {
		if competition <= 0 {
			return 0
		}
		return math.Inf(1)
	}
	return float64(competition) / float64(demand)
}

// OpportunityScore ranks a niche 0..100 from demand against saturation, scaled
// by how much relevant evidence backs it. Evidence scaling is what stops a
// niche with one lucky match from outranking a well-evidenced one.
//
// The score is 0 whenever there is no relevant evidence at all — an unknown
// niche scores zero, never "unmeasured but promising".
func OpportunityScore(demand, competition int, ev EvidenceSet) float64 {
	if ev.TotalEvidence() == 0 || demand <= 0 {
		return 0
	}
	sat := Saturation(demand, competition)
	if math.IsInf(sat, 1) {
		return 0
	}
	// Demand contributes on a log scale: the difference between 100 and 1,000
	// searches matters far more than between 100,000 and 101,000.
	demandScore := math.Log10(float64(demand)+1) / 6.0 // ~1.0 at 1M searches
	if demandScore > 1 {
		demandScore = 1
	}
	// Saturation penalty: 1.0 when uncrowded, decaying toward 0 as competition
	// per unit demand climbs past the documented low-competition ceiling.
	satScore := LowCompetitionSaturation / (LowCompetitionSaturation + sat)

	// Evidence coverage scales the whole thing, saturating at 10 relevant rows.
	coverage := math.Min(float64(ev.TotalEvidence())/10.0, 1.0)

	return math.Round(demandScore * satScore * coverage * 100)
}

// Confidence expresses how much the evidence supports the verdict, 0..1.
//
// #1492's contract, enforced here:
//   - 0 when no relevant evidence remains.
//   - never above MaxConfidenceWithoutKeywordEvidence when keyword evidence is zero.
//   - otherwise it rises with the share of returned rows that were actually
//     relevant, so a search that returned 20 rows of which 2 matched is not as
//     trustworthy as one where 18 matched.
func Confidence(ev EvidenceSet) float64 {
	if ev.TotalEvidence() == 0 {
		return 0
	}
	returned := ev.KeywordsReturned + ev.ProductsReturned
	if returned == 0 {
		return 0
	}
	// Share of everything we fetched that was actually on-target.
	precision := float64(ev.TotalEvidence()) / float64(returned)
	// Volume of evidence, saturating at 10 relevant rows.
	coverage := math.Min(float64(ev.TotalEvidence())/10.0, 1.0)

	conf := precision * coverage
	if ev.KeywordsRelevant == 0 && conf > MaxConfidenceWithoutKeywordEvidence {
		conf = MaxConfidenceWithoutKeywordEvidence
	}
	return math.Round(conf*100) / 100
}

// LowCompetition reports whether a niche may be labeled low-competition, and
// why not when it may not. The thresholds are the documented constants above;
// a label is never applied without both demand and competition evidence.
func LowCompetition(demand, competition int, ev EvidenceSet) (bool, string) {
	if ev.TotalEvidence() == 0 {
		return false, "no relevant evidence"
	}
	if !ev.hasSeedMetrics() {
		return false, "no demand/competition evidence for the seed itself"
	}
	if demand < MinDemandForLowCompetitionClaim {
		return false, "demand below the documented floor"
	}
	sat := Saturation(demand, competition)
	if sat > LowCompetitionSaturation {
		return false, "saturation above the documented low-competition ceiling"
	}
	return true, ""
}

// hasSeedMetrics reports whether the seed's own demand/competition pair was
// present. Without it, "low competition" is an unsupported claim.
func (e EvidenceSet) hasSeedMetrics() bool { return e.KeywordsReturned > 0 }

// ComputePriceBand summarizes prices across the given products. Only products
// passing the caller's filter should be handed in; the band reports how many
// rows it was computed from so a one-listing "median" is visible as such.
func ComputePriceBand(products []ProductRow) PriceBand {
	prices := make([]float64, 0, len(products))
	for _, p := range products {
		if p.Price > 0 {
			prices = append(prices, p.Price)
		}
	}
	if len(prices) == 0 {
		return PriceBand{}
	}
	sort.Float64s(prices)
	return PriceBand{
		Min:    prices[0],
		Median: median(prices),
		Max:    prices[len(prices)-1],
		Count:  len(prices),
	}
}

// median returns the median of a pre-sorted slice.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
