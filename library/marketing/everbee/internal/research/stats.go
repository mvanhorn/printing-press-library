// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"math"
	"sort"
	"strings"
)

// CompetitorSample is the market shape of a niche: who is already selling here
// and how entrenched they are. Every statistic is computed from the raw rows,
// which are returned alongside so the numbers can be checked.
type CompetitorSample struct {
	Seed          string       `json:"seed"`
	ResultCount   int          `json:"result_count"` // total matching listings upstream reports
	SampledRows   int          `json:"sampled_rows"` // rows the statistics were computed from
	MedianPrice   float64      `json:"median_price"`
	PriceBand     PriceBand    `json:"price_band"`
	ReviewDensity float64      `json:"review_density"` // mean reviews per listing
	SalesDensity  float64      `json:"sales_density"`  // mean estimated monthly sales per listing
	ListingAge    AgeQuartile  `json:"listing_age_months"`
	TopShops      []ShopStat   `json:"top_shops"`
	Evidence      EvidenceSet  `json:"evidence"`
	Warnings      []string     `json:"warnings"`
	Products      []ProductRow `json:"products"`
	Provenance    []Provenance `json:"provenance"`
}

// AgeQuartile summarizes how established the competing listings are. A niche
// whose listings are all months old is harder to enter than one where the
// median listing appeared last week.
type AgeQuartile struct {
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	Count  int     `json:"count"`
}

// ShopStat is one competitor shop's footprint in the niche.
type ShopStat struct {
	ShopName     string  `json:"shop_name"`
	Listings     int     `json:"listings_in_niche"`
	MedianPrice  float64 `json:"median_price"`
	TotalRevenue float64 `json:"est_mo_revenue"`
}

// SampleCompetitors computes market-shape statistics over the products that are
// both relevant to the seed and of the requested type. Rows that fail either
// test are excluded from the statistics (they would misdescribe the market) but
// are still returned to the caller.
func SampleCompetitors(seed string, products []ProductRow, listingCount int, opts Options) CompetitorSample {
	minRel := opts.minRelevance()

	s := CompetitorSample{
		Seed:        seed,
		ResultCount: listingCount,
		Products:    products,
		Warnings:    []string{},
	}

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
	s.Evidence = ev
	s.SampledRows = len(inScope)

	if len(inScope) == 0 {
		s.Warnings = append(s.Warnings,
			"no listings matched the seed and product type, so no market statistics were computed; this is an empty result, not a zero-competition market.")
		return s
	}

	s.PriceBand = ComputePriceBand(inScope)
	s.MedianPrice = s.PriceBand.Median

	var reviews, sales float64
	ages := make([]float64, 0, len(inScope))
	for _, p := range inScope {
		reviews += float64(p.ReviewCount)
		sales += p.EstMoSales
		if p.ListingAgeMonth > 0 {
			ages = append(ages, p.ListingAgeMonth)
		}
	}
	s.ReviewDensity = round2(reviews / float64(len(inScope)))
	s.SalesDensity = round2(sales / float64(len(inScope)))
	s.ListingAge = quartiles(ages)
	s.TopShops = topShops(inScope)

	if len(ages) == 0 {
		s.Warnings = append(s.Warnings, "EverBee returned no listing-age data for these rows; listing-age quartiles are empty.")
	}
	return s
}

func quartiles(v []float64) AgeQuartile {
	if len(v) == 0 {
		return AgeQuartile{}
	}
	sort.Float64s(v)
	return AgeQuartile{
		P25:    round2(percentile(v, 0.25)),
		Median: round2(median(v)),
		P75:    round2(percentile(v, 0.75)),
		Count:  len(v),
	}
}

// percentile returns the p-th percentile of a pre-sorted slice using nearest-rank.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// topShops rolls the in-scope listings up by shop, ranked by how much of the
// niche's estimated revenue each shop holds.
func topShops(products []ProductRow) []ShopStat {
	byShop := map[string][]ProductRow{}
	for _, p := range products {
		name := strings.TrimSpace(p.ShopName)
		if name == "" {
			continue
		}
		byShop[name] = append(byShop[name], p)
	}
	out := make([]ShopStat, 0, len(byShop))
	for name, rows := range byShop {
		var rev float64
		for _, r := range rows {
			rev += r.EstMoRevenue
		}
		out = append(out, ShopStat{
			ShopName:     name,
			Listings:     len(rows),
			MedianPrice:  ComputePriceBand(rows).Median,
			TotalRevenue: round2(rev),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalRevenue != out[j].TotalRevenue {
			return out[i].TotalRevenue > out[j].TotalRevenue
		}
		return out[i].ShopName < out[j].ShopName
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
