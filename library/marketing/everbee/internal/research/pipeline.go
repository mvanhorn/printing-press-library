// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Options controls a niche run.
type Options struct {
	Seed          string
	ProductType   string  // "", "physical", "digital", "apparel"
	ExcludeSVGPNG bool    // drop digital cut-file listings from the evidence count
	MinRelevance  float64 // evidence floor; rows below it are returned but not counted
	PerPage       int     // product rows to request
}

func (o Options) minRelevance() float64 {
	if o.MinRelevance <= 0 {
		return DefaultMinRelevance
	}
	return o.MinRelevance
}

// Niche runs the evidence-aware pipeline for one seed: seeded keyword search +
// seeded product search, then local scoring.
//
// It returns a Verdict even when EverBee returns nothing relevant — an empty
// result with confidence 0 and an explicit warning is the honest answer, and is
// what lets a caller distinguish "no opportunity here" from "the tool broke".
// The only errors returned are transport failures and plan-cap refusals, both
// of which mean "we could not look", not "we looked and found nothing".
func Niche(ctx context.Context, c APIDoer, opts Options) (*Verdict, error) {
	seed := strings.TrimSpace(opts.Seed)
	if seed == "" {
		return nil, errors.New("seed keyword is required")
	}

	v := &Verdict{
		Seed:        seed,
		ProductType: opts.ProductType,
		Warnings:    []string{},
		Keywords:    []KeywordRow{},
		Products:    []ProductRow{},
		Provenance:  []Provenance{},
	}

	keywords, seedMetrics, kwProv, kwErr := FetchKeywords(ctx, c, seed, 1)
	if kwErr != nil {
		// A plan cap is a refusal to answer, not an empty answer. Surface it.
		if errors.Is(kwErr, ErrPlanCapReached) {
			return nil, kwErr
		}
		v.Warnings = append(v.Warnings, fmt.Sprintf("keyword evidence unavailable: %v", kwErr))
		kwProv.Fallback = "keyword search failed; verdict rests on product evidence only"
	}
	v.Provenance = append(v.Provenance, kwProv)
	if keywords == nil {
		keywords = []KeywordRow{}
	}
	v.Keywords = keywords
	v.SeedMetrics = seedMetrics

	products, listingCount, prProv, prErr := FetchProducts(ctx, c, seed, opts.PerPage)
	if prErr != nil {
		if errors.Is(prErr, ErrPlanCapReached) {
			return nil, prErr
		}
		v.Warnings = append(v.Warnings, fmt.Sprintf("product evidence unavailable: %v", prErr))
		prProv.Fallback = "product search failed; verdict rests on keyword evidence only"
	}
	v.Provenance = append(v.Provenance, prProv)
	if products == nil {
		products = []ProductRow{}
	}
	v.Products = products
	v.ListingCount = listingCount

	// Both legs failed: we did not look at all. That is a broken research path,
	// not an empty market, and it must not be reported as a zero-evidence verdict
	// — a caller would read "no opportunity here" from what is really "the API
	// refused us" (an expired token being the common case).
	if kwErr != nil && prErr != nil {
		return nil, fmt.Errorf("research path failed for %q: neither keyword nor product evidence could be fetched: %w", seed, kwErr)
	}

	minRel := opts.minRelevance()

	ev := EvidenceSet{
		KeywordsReturned: len(keywords),
		ProductsReturned: len(products),
	}
	for _, k := range keywords {
		if k.Relevance >= minRel {
			ev.KeywordsRelevant++
		}
	}
	// Products count as evidence only when they are BOTH relevant to the seed
	// and of the requested product type. A cut-file SVG is not evidence that an
	// apparel niche is viable.
	relevantInType := make([]ProductRow, 0, len(products))
	for _, p := range products {
		if p.Relevance < minRel {
			continue
		}
		ev.ProductsRelevant++
		if ProductTypeMatch(p, opts.ProductType, opts.ExcludeSVGPNG) {
			ev.ProductsInType++
			relevantInType = append(relevantInType, p)
		}
	}
	v.Evidence = ev

	v.Demand = seedMetrics.Volume
	v.Competition = seedMetrics.Competition
	v.Saturation = finiteOrNil(Saturation(v.Demand, v.Competition))
	v.PriceBand = ComputePriceBand(relevantInType)
	v.Opportunity = OpportunityScore(v.Demand, v.Competition, ev)
	v.Confidence = Confidence(ev)

	v.Warnings = append(v.Warnings, warnings(v, ev, minRel)...)

	return v, nil
}

// warnings states plainly what the evidence does not support. Every claim this
// verdict cannot make gets said out loud rather than left for the caller to infer.
func warnings(v *Verdict, ev EvidenceSet, minRel float64) []string {
	var out []string

	if ev.TotalEvidence() == 0 {
		out = append(out, fmt.Sprintf(
			"no evidence: EverBee returned %d keywords and %d listings for %q, none scoring at or above the %.2f relevance floor. Confidence is 0 and no opportunity is claimed.",
			ev.KeywordsReturned, ev.ProductsReturned, v.Seed, minRel))
		return out
	}
	if ev.KeywordsRelevant == 0 {
		out = append(out, fmt.Sprintf(
			"no relevant keyword evidence (%d returned, none at or above the %.2f floor); confidence is capped at %.1f.",
			ev.KeywordsReturned, minRel, MaxConfidenceWithoutKeywordEvidence))
	}
	if !v.SeedMetrics.Present {
		out = append(out, "EverBee returned no demand/competition metrics for the seed itself; demand, saturation, and any low-competition claim are unsupported.")
	}
	if ev.ProductsRelevant > 0 && ev.ProductsInType == 0 && v.ProductType != "" {
		out = append(out, fmt.Sprintf(
			"%d relevant listings found but none matched product type %q; the price band is empty.",
			ev.ProductsRelevant, v.ProductType))
	}
	if low, why := LowCompetition(v.Demand, v.Competition, ev); low {
		out = append(out, fmt.Sprintf(
			"low competition: saturation %.1f is at or below the documented ceiling of %.0f competing listings per unit demand, with %d relevant evidence rows.",
			Saturation(v.Demand, v.Competition), LowCompetitionSaturation, ev.TotalEvidence()))
	} else if why != "" {
		out = append(out, "not labeled low-competition: "+why+".")
	}
	return out
}

// SubNiche is one child niche in a batch run, carrying its own verdict.
type SubNiche struct {
	Seed        string   `json:"seed"`
	Demand      int      `json:"demand"`
	Competition int      `json:"competition"`
	Saturation  *float64 `json:"saturation"`
	Opportunity float64  `json:"opportunity_score"`
	// NormalizedScore rescales Opportunity across the batch so children are
	// comparable to each other, which a raw score across different demand
	// magnitudes is not.
	NormalizedScore float64     `json:"normalized_score"`
	Confidence      float64     `json:"confidence"`
	Evidence        EvidenceSet `json:"evidence"`
	PriceBand       PriceBand   `json:"price_band"`
	Warnings        []string    `json:"warnings"`
	Error           string      `json:"error,omitempty"`
}

// Normalize rescales opportunity scores across a batch to 0..100 relative to
// the best-scoring child, so children can be ranked against each other.
// Children that errored keep a zero score and are never treated as a floor.
func Normalize(subs []SubNiche) {
	best := 0.0
	for _, s := range subs {
		if s.Error == "" && s.Opportunity > best {
			best = s.Opportunity
		}
	}
	for i := range subs {
		if best <= 0 || subs[i].Error != "" {
			subs[i].NormalizedScore = 0
			continue
		}
		subs[i].NormalizedScore = round1(subs[i].Opportunity / best * 100)
	}
	sort.SliceStable(subs, func(i, j int) bool {
		return subs[i].NormalizedScore > subs[j].NormalizedScore
	})
}

// finiteOrNil returns nil for values JSON cannot encode (Inf/NaN), so an
// undefined statistic marshals as null instead of crashing the command or, worse,
// being coerced to a misleading zero.
func finiteOrNil(f float64) *float64 {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil
	}
	return &f
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ChildSeeds picks child seeds for a batch run out of EverBee's own keyword
// suggestions for the parent, keeping only suggestions that are actually
// relevant to the parent and, when a product word is given, that mention it.
//
// Using EverBee's suggestion engine rather than inventing permutations is what
// keeps the children real: these are keywords Etsy shoppers actually search.
func ChildSeeds(rows []KeywordRow, parent, productWord string, minRelevance float64, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	if minRelevance <= 0 {
		minRelevance = DefaultMinRelevance
	}
	seen := map[string]bool{}
	out := make([]string, 0, limit)
	for _, r := range rows {
		kw := strings.TrimSpace(r.Keyword)
		if kw == "" || seen[strings.ToLower(kw)] {
			continue
		}
		if Relevance(parent, kw) < minRelevance {
			continue
		}
		if productWord != "" && Relevance(productWord, kw) == 0 {
			continue
		}
		seen[strings.ToLower(kw)] = true
		out = append(out, kw)
		if len(out) >= limit {
			break
		}
	}
	return out
}
