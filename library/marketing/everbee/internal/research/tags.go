// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"math"
	"sort"
	"strings"
)

// Seasonality thresholds. Documented rather than magic: a seasonal-vs-evergreen
// call has to be defensible, and these are the numbers behind it.
const (
	// SeasonalCoefficientOfVariation is the coefficient of variation (stddev /
	// mean) above which a keyword's trend is called seasonal. A keyword whose
	// monthly search volume swings widely across the year is a spike, not a
	// durable market.
	SeasonalCoefficientOfVariation = 0.45

	// MinTrendPointsForSeasonality is the minimum number of trend points needed
	// before any seasonality call is made at all. Below this, the honest answer
	// is "unknown", not "evergreen".
	MinTrendPointsForSeasonality = 6
)

// TagConsensus is what the winning listings in a niche agree on: the tags and
// title words they share, and whether the demand behind them is seasonal.
//
// EverBee's own Tag Analytics tab computes this client-side from the tags that
// ride on each product row (the browser capture confirmed the tab fires no
// network request), so this is the same derivation, made scriptable.
type TagConsensus struct {
	Seed        string       `json:"seed"`
	SampledRows int          `json:"sampled_rows"`
	Tags        []TagStat    `json:"tags"`
	TitleTokens []TagStat    `json:"title_tokens"`
	Seasonality Seasonality  `json:"seasonality"`
	Evidence    EvidenceSet  `json:"evidence"`
	Warnings    []string     `json:"warnings"`
	Provenance  []Provenance `json:"provenance"`
}

// TagStat is one tag or title token and how widely the niche's listings use it.
type TagStat struct {
	Value string  `json:"value"`
	Count int     `json:"count"`
	Share float64 `json:"share"` // fraction of sampled listings carrying it, 0..1
}

// Seasonality classifies demand as seasonal or evergreen, or declines to call
// it when there is not enough trend data. "unknown" is a real answer here.
type Seasonality struct {
	Verdict     string  `json:"verdict"` // "seasonal" | "evergreen" | "unknown"
	Variation   float64 `json:"coefficient_of_variation"`
	TrendPoints int     `json:"trend_points"`
	Reason      string  `json:"reason"`
}

// ConsensusTags aggregates the tags and title tokens across the listings that
// are relevant to the seed and of the requested type.
func ConsensusTags(seed string, products []ProductRow, opts Options, trend []float64) TagConsensus {
	minRel := opts.minRelevance()

	tc := TagConsensus{Seed: seed, Warnings: []string{}}

	inScope := make([]ProductRow, 0, len(products))
	ev := EvidenceSet{ProductsReturned: len(products)}
	for _, p := range products {
		if p.Relevance < minRel {
			continue
		}
		ev.ProductsRelevant++
		if !ProductTypeMatch(p, opts.ProductType, opts.ExcludeSVGPNG) {
			continue
		}
		ev.ProductsInType++
		inScope = append(inScope, p)
	}
	tc.Evidence = ev
	tc.SampledRows = len(inScope)
	tc.Seasonality = classifySeasonality(trend)

	if len(inScope) == 0 {
		tc.Warnings = append(tc.Warnings,
			"no listings matched the seed and product type, so there is no tag consensus to report.")
		return tc
	}

	tagCounts := map[string]int{}
	tokenCounts := map[string]int{}
	for _, p := range inScope {
		// Count each tag once per listing, so a listing that repeats a tag does
		// not inflate the consensus.
		seenTag := map[string]bool{}
		for _, tag := range p.Tags {
			t := strings.ToLower(strings.TrimSpace(tag))
			if t == "" || seenTag[t] {
				continue
			}
			seenTag[t] = true
			tagCounts[t]++
		}
		seenTok := map[string]bool{}
		for _, tok := range Tokenize(p.Title) {
			if seenTok[tok] {
				continue
			}
			seenTok[tok] = true
			tokenCounts[tok]++
		}
	}

	tc.Tags = rankStats(tagCounts, len(inScope), 25)
	tc.TitleTokens = rankStats(tokenCounts, len(inScope), 25)

	if len(tc.Tags) == 0 {
		tc.Warnings = append(tc.Warnings, "EverBee returned no tags on these listings; tag consensus is empty.")
	}
	return tc
}

func rankStats(counts map[string]int, total, limit int) []TagStat {
	out := make([]TagStat, 0, len(counts))
	for v, c := range counts {
		out = append(out, TagStat{
			Value: v,
			Count: c,
			Share: round2(float64(c) / float64(total)),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// classifySeasonality calls seasonal vs evergreen from the variability of a
// keyword's trend series. With too few points it returns "unknown" rather than
// guessing — an unsupported evergreen call is exactly the kind of confident
// nonsense this CLI exists to avoid.
func classifySeasonality(trend []float64) Seasonality {
	pts := make([]float64, 0, len(trend))
	for _, v := range trend {
		if v > 0 {
			pts = append(pts, v)
		}
	}
	if len(pts) < MinTrendPointsForSeasonality {
		return Seasonality{
			Verdict:     "unknown",
			TrendPoints: len(pts),
			Reason:      "not enough trend data from EverBee to judge seasonality",
		}
	}
	var sum float64
	for _, v := range pts {
		sum += v
	}
	mean := sum / float64(len(pts))
	if mean == 0 {
		return Seasonality{Verdict: "unknown", TrendPoints: len(pts), Reason: "trend series is all zeros"}
	}
	var sq float64
	for _, v := range pts {
		d := v - mean
		sq += d * d
	}
	stddev := math.Sqrt(sq / float64(len(pts)))
	cv := stddev / mean

	s := Seasonality{Variation: round2(cv), TrendPoints: len(pts)}
	if cv > SeasonalCoefficientOfVariation {
		s.Verdict = "seasonal"
		s.Reason = "search volume swings more across the year than the documented seasonal threshold"
	} else {
		s.Verdict = "evergreen"
		s.Reason = "search volume is stable across the year, below the documented seasonal threshold"
	}
	return s
}
