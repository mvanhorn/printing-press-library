// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

// Package carsource holds the hand-built HTML source clients for Rental Car Spain.
//
// Both sources (DoYouSpain, the aggregator, and Delpaso, a direct Málaga
// supplier) serve server-rendered HTML rather than a JSON API, so these
// clients live outside the generated internal/client package. They speak
// plain net/http with a per-request cookie jar and a non-browser User-Agent
// (DoYouSpain's WAF returns HTTP 406 for spoofed Chrome clients).
package carsource

// UserAgent is the non-browser User-Agent every source client must send.
// DoYouSpain's Signal Sciences WAF returns 406 when a browser UA is sent over
// a non-browser TLS stack; a plain token like this returns 200.
const UserAgent = "rentalcarspain-pp-cli/0.1 (+https://github.com/zetabytelab)"

// Location is a resolved DoYouSpain pickup/dropoff place.
type Location struct {
	Code        string `json:"code"`         // DoYouSpain internal code, e.g. MAL02
	Description string `json:"description"`  // human name, e.g. "Malaga Airport (AGP)"
	Country     string `json:"country"`      // ISO country, e.g. ES
	IATA        string `json:"iata,omitempty"`
}

// Offer is one rental quote, normalized across sources.
type Offer struct {
	Source        string  `json:"source"`         // "doyouspain" | "delpaso"
	Supplier      string  `json:"supplier"`       // human name, e.g. "Delpaso"
	SupplierCode  string  `json:"supplier_code"`  // source-native code, e.g. "PAS"
	Car           string  `json:"car"`            // model, e.g. "Citroen DS3, Special Offer"
	CarClass      string  `json:"car_class"`      // category, e.g. "Small Cars"
	Seats         int     `json:"seats,omitempty"`
	Doors         int     `json:"doors,omitempty"`
	Transmission  string  `json:"transmission,omitempty"` // "Manual" | "Automatic"
	FuelPolicy    string  `json:"fuel_policy,omitempty"`
	Mileage       string  `json:"mileage,omitempty"`
	PerDay        float64 `json:"per_day"`        // current per-day price
	Total         float64 `json:"total"`          // current total for the rental
	BaseTotal     float64 `json:"base_total,omitempty"` // struck-through original, when present
	Currency      string  `json:"currency"`       // "EUR" | "GBP"
	// FullInsurance reports whether the quoted price is fully insured with no
	// excess (zero-excess / total coverage). For DoYouSpain offers this is
	// derived from the parsed excess: the listed price includes CDW + Theft
	// Protection but usually keeps an excess, so most aggregator offers are
	// NOT full insurance. Delpaso's own quotes are total coverage (no excess).
	FullInsurance bool    `json:"full_insurance"`
	// Excess is the insurance excess / deductible the renter is liable for
	// (and typically must leave as a deposit) when it is stated in the offer.
	// ExcessKnown distinguishes a genuine zero excess from an unparsed one.
	Excess        float64 `json:"excess"`
	ExcessKnown   bool    `json:"excess_known"`
	// FullInsuranceTotal is the price to rent this car fully insured with no
	// excess (base rate plus the source's zero-excess add-on), when the source
	// exposes it. Direct-supplier offers set this equal to Total (already
	// zero-excess); it is 0/omitted when the source does not price full cover.
	FullInsuranceTotal float64 `json:"full_insurance_total,omitempty"`
	Deposit       float64 `json:"deposit,omitempty"`
	// MinAge is the minimum driver age eligible for this car/supplier, when the
	// source states it (Clickrent gates each car 18/21/30; Goldcar declines under
	// 21). 0 means unknown / no stated restriction. Used to flag offers a young
	// driver cannot actually rent.
	MinAge        int     `json:"min_age,omitempty"`
	// SupplierScore is the supplier's customer rating (0–10) at this airport,
	// and Reviews the rating count. Populated from Rentalcars depot ratings
	// (primary) and DoYouSpain supplier scores.
	SupplierScore float64 `json:"supplier_score,omitempty"`
	Reviews       int     `json:"reviews,omitempty"`
	// RatingCategories carries the per-category breakdown (0–10) when the source
	// exposes it (Rentalcars: cleanliness, condition, collectTime, dropOffTime,
	// efficiency, locating, valForMoney).
	RatingCategories map[string]float64 `json:"rating_categories,omitempty"`
	FreeCancel    bool    `json:"free_cancellation,omitempty"`
	URL           string  `json:"url,omitempty"`
}

// SearchQuery describes a DoYouSpain search.
type SearchQuery struct {
	LocationCode  string // pickup code, e.g. MAL02
	DropoffCode   string // dropoff code; defaults to LocationCode when empty
	Pickup        string // dd/mm/yyyy
	Dropoff       string // dd/mm/yyyy
	PickupTime    string // HH:MM (30-min steps); defaults to 10:00
	DropoffTime   string // HH:MM; defaults to 10:00
	DriverAge     int    // defaults to 35
	Language      string // defaults to en
}
