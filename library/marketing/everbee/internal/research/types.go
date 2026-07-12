// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// Package research turns EverBee's raw keyword and product responses into
// evidence-annotated research results.
//
// The contract every type here exists to enforce: a score is never reported
// without the evidence that produced it. EverBee's API will answer a question
// you did not ask — its default suggestion feeds return unranked filler
// regardless of the seed — so a caller that trusts transport success alone ends
// up with a confident-looking verdict built on unrelated rows. Every result
// carries an EvidenceSet describing what was actually fetched, how much of it
// was relevant to the seed, and where it came from.
package research

import "time"

// Provenance records where a metric came from, so a verdict can be audited
// rather than trusted. Every research result carries one.
type Provenance struct {
	Source     string    `json:"source"`             // e.g. "everbee:keyword_suggestion"
	Endpoint   string    `json:"endpoint"`           // the API path actually called
	QueryScope string    `json:"query_scope"`        // the seed/term that was sent
	FetchedAt  time.Time `json:"fetched_at"`         // when the upstream call returned
	Fallback   string    `json:"fallback,omitempty"` // set when a degraded path was used
}

// KeywordRow is one keyword returned by EverBee's seeded keyword search,
// annotated with its relevance to the seed.
type KeywordRow struct {
	Keyword     string  `json:"keyword"`
	Volume      int     `json:"volume"`
	Competition int     `json:"competition"`
	Score       float64 `json:"score"`
	CPC         float64 `json:"cpc,omitempty"`
	// Trend is EverBee's search-volume series for the keyword, when it supplies
	// one. Frequently empty — seasonality degrades to "unknown" rather than
	// inventing a series.
	Trend []float64 `json:"trend,omitempty"`
	// Relevance is token-overlap against the seed, 0..1. Rows are annotated,
	// never dropped: the caller filters with --min-relevance.
	Relevance float64 `json:"relevance"`
}

// ProductRow is one Etsy listing returned by EverBee's seeded product search,
// annotated with its relevance to the seed.
type ProductRow struct {
	ListingID       int64    `json:"listing_id"`
	Title           string   `json:"title"`
	ShopName        string   `json:"shop_name,omitempty"`
	Price           float64  `json:"price"`
	ListingType     string   `json:"listing_type,omitempty"` // "physical" | "download"
	Tags            []string `json:"tags,omitempty"`
	EstMoRevenue    float64  `json:"est_mo_revenue"`
	EstMoSales      float64  `json:"est_mo_sales"`
	EstTotalSales   float64  `json:"est_total_sales"`
	ReviewCount     int      `json:"review_count"`
	ListingAgeMonth float64  `json:"listing_age_months"`
	URL             string   `json:"url,omitempty"`
	Relevance       float64  `json:"relevance"`
}

// EvidenceSet is the audit trail behind a score. Counts are reported even when
// zero — an empty EvidenceSet is a meaningful, honest answer, not a failure.
type EvidenceSet struct {
	KeywordsReturned int `json:"keywords_returned"`
	KeywordsRelevant int `json:"keywords_relevant"`
	ProductsReturned int `json:"products_returned"`
	ProductsRelevant int `json:"products_relevant"`
	// ProductsInType counts relevant products that also matched the requested
	// product type (physical/digital/apparel). Equals ProductsRelevant when no
	// type constraint was applied.
	ProductsInType int `json:"products_in_type"`
}

// TotalEvidence is the count of relevant rows backing a verdict.
func (e EvidenceSet) TotalEvidence() int {
	return e.KeywordsRelevant + e.ProductsRelevant
}

// SeedMetrics are EverBee's own metrics for the seed itself, returned in the
// keyword response's `searched_keyword` block. This is the demand-vs-competition
// pair that makes a "low competition" claim defensible.
type SeedMetrics struct {
	Keyword     string `json:"keyword"`
	Volume      int    `json:"volume"`
	Competition int    `json:"competition"`
	Present     bool   `json:"present"`
}

// Verdict is the evidence-aware result for a single niche.
type Verdict struct {
	Seed        string      `json:"seed"`
	ProductType string      `json:"product_type,omitempty"`
	SeedMetrics SeedMetrics `json:"seed_metrics"`
	Demand      int         `json:"demand"`      // seed search volume
	Competition int         `json:"competition"` // competing listing count
	// Saturation is competition per unit demand. It is null when demand is zero:
	// saturation is undefined there, and reporting 0 would read as "uncrowded".
	Saturation   *float64     `json:"saturation"`
	ListingCount int          `json:"listing_count"` // total matching listings upstream reports
	PriceBand    PriceBand    `json:"price_band"`
	Opportunity  float64      `json:"opportunity_score"` // 0..100
	Confidence   float64      `json:"confidence"`        // 0..1, tied to evidence coverage
	Evidence     EvidenceSet  `json:"evidence"`
	Warnings     []string     `json:"warnings"`
	Keywords     []KeywordRow `json:"keywords"`
	Products     []ProductRow `json:"products"`
	Provenance   []Provenance `json:"provenance"`
}

// PriceBand summarizes the price distribution of the matching listings.
type PriceBand struct {
	Min    float64 `json:"min"`
	Median float64 `json:"median"`
	Max    float64 `json:"max"`
	Count  int     `json:"count"`
}
