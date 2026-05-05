// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

// Package redfin provides Redfin-specific types, response parsers, and
// schema migrations layered on top of the generic store.
package redfin

// Listing is the canonical, hand-built shape we project Stingray search and
// detail responses into. Fields populated by the gis (search) endpoint differ
// from those populated by the home/details/* endpoints; see ParseSearchResponse
// and ParseListingDetail for the per-source contracts.
type Listing struct {
	URL          string              `json:"url"`
	PropertyID   int64               `json:"property_id"`
	ListingID    int64               `json:"listing_id,omitempty"`
	MLS          string              `json:"mls,omitempty"`
	Status       string              `json:"status,omitempty"`
	Address      Address             `json:"address"`
	Price        int                 `json:"price,omitempty"`
	Beds         float64             `json:"beds,omitempty"`
	Baths        float64             `json:"baths,omitempty"`
	Sqft         int                 `json:"sqft,omitempty"`
	LotSize      int                 `json:"lot_size,omitempty"`
	YearBuilt    int                 `json:"year_built,omitempty"`
	PropertyType string              `json:"property_type,omitempty"`
	HOA          int                 `json:"hoa,omitempty"`
	DOM          int                 `json:"dom,omitempty"`
	ListedAt     string              `json:"listed_at,omitempty"`
	SoldAt       string              `json:"sold_at,omitempty"`
	Estimate     int                 `json:"estimate,omitempty"`
	Photos       []string            `json:"photos,omitempty"`
	PriceHistory []PriceHistoryEvent `json:"price_history,omitempty"`
	Schools      []School            `json:"schools,omitempty"`
	SearchSlug   string              `json:"search_slug,omitempty"`
}

// Address is the structured location for a Listing. Latitude/Longitude are
// emitted by Stingray's gis response; street/city/state/postal come from the
// home/details/initialInfo address block.
type Address struct {
	Street     string  `json:"street,omitempty"`
	City       string  `json:"city,omitempty"`
	State      string  `json:"state,omitempty"`
	PostalCode string  `json:"postal_code,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
}

// PriceHistoryEvent is one row of Redfin's listing-history ledger.
type PriceHistoryEvent struct {
	Date   string `json:"date"`
	Event  string `json:"event"`
	Price  int    `json:"price,omitempty"`
	Source string `json:"source,omitempty"`
}

// School is one school tied to a listing's attendance zone.
type School struct {
	Name   string  `json:"name"`
	Grades string  `json:"grades,omitempty"`
	Rating float64 `json:"rating,omitempty"`
}

// RegionTrendPoint is one row of an aggregate-trends long-format table.
// Region and RegionID are filled by the caller (the trends endpoint doesn't
// echo them in the rows themselves).
type RegionTrendPoint struct {
	Region   string  `json:"region"`
	RegionID int64   `json:"region_id,omitempty"`
	Month    string  `json:"month"`
	Metric   string  `json:"metric"`
	Value    float64 `json:"value"`
}

// SearchOptions is the gis-endpoint query model. Zero-valued fields are
// dropped by BuildSearchParams so callers can pass only the filters they
// actually want.
type SearchOptions struct {
	RegionID        int64
	RegionType      int
	Status          int
	SoldFlags       string
	UIPropertyTypes []int
	BedsMin         float64
	BathsMin        float64
	PriceMin        int
	PriceMax        int
	SqftMin         int
	SqftMax         int
	YearMin         int
	YearMax         int
	LotMin          int
	SchoolsMin      int
	Polygon         string
	NumHomes        int
	PageNumber      int
	Sort            string
}
