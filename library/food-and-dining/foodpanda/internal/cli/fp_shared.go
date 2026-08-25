// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for foodpanda novel commands.
// Kept in its own file so `generate --force` preserves it verbatim.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
)

const (
	fpDiscoVendors = "https://disco.deliveryhero.io/listing/api/v1/pandora/vendors"
	fpDiscoSearch  = "https://disco.deliveryhero.io/listing/api/v1/pandora/search"
	fpAddresses    = "https://pk.fd-api.com/api/v5/customers/addresses"
	// fpPageSize is the largest page the disco listing endpoint honours.
	fpPageSize = 48
)

// fpMarkets is the set of foodpanda markets verified reachable during discovery.
var fpMarkets = []string{"pk", "bd", "sg", "my", "hk", "th"}

// fpVendorHost returns the market-scoped vendor-detail host. Vendor codes are
// market-scoped: a Pakistani code 404s against the Singapore backend.
func fpVendorHost(country string) string {
	return fmt.Sprintf("https://%s.fd-api.com/api/v5/vendors", strings.ToLower(country))
}

func fpReviewsHost(country string) string {
	return fmt.Sprintf("https://reviews-api-%s.fd-api.com/reviews/vendor", strings.ToLower(country))
}

// fpVendor is the subset of the ~80-field disco vendor object this CLI uses.
type fpVendor struct {
	ID                  float64 `json:"id"`
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	Address             string  `json:"address"`
	Rating              float64 `json:"rating"`
	ReviewNumber        float64 `json:"review_number"`
	Distance            float64 `json:"distance"`
	MinDeliveryFee      float64 `json:"minimum_delivery_fee"`
	MinOrderAmount      float64 `json:"minimum_order_amount"`
	MinDeliveryTime     float64 `json:"minimum_delivery_time"`
	ServiceFeePct       float64 `json:"service_fee_percentage_amount"`
	ServiceTaxPct       float64 `json:"service_tax_percentage_amount"`
	VatPct              float64 `json:"vat_percentage_amount"`
	LoyaltyPct          float64 `json:"loyalty_percentage_amount"`
	Budget              float64 `json:"budget"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	URLKey              string  `json:"url_key"`
	Vertical            string  `json:"vertical"`
	IsActive            bool    `json:"is_active"`
	IsPromoted          bool    `json:"is_promoted"`
	IsPremium           bool    `json:"is_premium"`
	PremiumPosition     float64 `json:"premium_position"`
	VendorPoints        float64 `json:"vendor_points"`
	IsPreferredPartner  bool    `json:"is_preferred_partner"`
	NCRPricingModel     string  `json:"ncr_pricing_model"`
	NCRToken            string  `json:"ncr_token"`
	IsDeliveryEnabled   bool    `json:"is_delivery_enabled"`
	IsNew               bool    `json:"is_new"`
	IsBestInCity        bool    `json:"is_best_in_city"`
	HasDeliveryProvider bool    `json:"has_delivery_provider"`
	DeliveryProvider    string  `json:"delivery_provider"`
	Tag                 string  `json:"tag"`
	Cuisines            []struct {
		ID   float64 `json:"id"`
		Name string  `json:"name"`
	} `json:"cuisines"`
}

// PrimaryCuisine returns the first cuisine name, or "" when none is present.
func (v fpVendor) PrimaryCuisine() string {
	if len(v.Cuisines) == 0 {
		return ""
	}
	return v.Cuisines[0].Name
}

// AdBuyer reports whether the vendor participates in foodpanda's
// non-commission-revenue (advertising) product. This is an ad-spend signal, NOT
// a merchant commission rate — commission is not exposed in any consumer
// surface.
func (v fpVendor) AdBuyer() bool {
	return v.NCRPricingModel != "" || v.NCRToken != "" || v.IsPromoted
}

type fpDiscoEnvelope struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       struct {
		AvailableCount int        `json:"available_count"`
		ReturnedCount  int        `json:"returned_count"`
		Items          []fpVendor `json:"items"`
	} `json:"data"`
}

// fpQuery holds the geo/market scope every listing call needs.
type fpQuery struct {
	Lat      float64
	Lng      float64
	Country  string
	Vertical string
	Sort     string
	Query    string
}

func (q fpQuery) params(limit, offset int) map[string]string {
	p := map[string]string{
		"latitude":      strconv.FormatFloat(q.Lat, 'f', -1, 64),
		"longitude":     strconv.FormatFloat(q.Lng, 'f', -1, 64),
		"language_id":   "1",
		"country":       q.Country,
		"vertical":      q.Vertical,
		"limit":         strconv.Itoa(limit),
		"offset":        strconv.Itoa(offset),
		"configuration": "Variant1",
		"customer_type": "regular",
		"include":       "characteristics",
	}
	if q.Sort != "" {
		p["sort"] = q.Sort
	}
	if q.Query != "" {
		p["query"] = q.Query
	}
	return p
}

func (q *fpQuery) normalize() {
	if q.Country == "" {
		q.Country = "pk"
	}
	q.Country = strings.ToLower(q.Country)
	if q.Vertical == "" {
		q.Vertical = "restaurants"
	}
}

// fpSweepResult carries both the vendors and honest scan accounting, so an
// empty result is distinguishable from "we stopped scanning early".
type fpSweepResult struct {
	Vendors     []fpVendor
	Scanned     int
	Available   int
	PagesRead   int
	ScanCapHit  bool
	MaxScanPage int
}

// fpSweep pages the disco listing endpoint, bounding scan effort separately
// from how many rows the caller keeps. Set url to fpDiscoSearch for text search.
func fpSweep(ctx context.Context, c *client.Client, url string, q fpQuery, maxScanPages int) (*fpSweepResult, error) {
	q.normalize()
	if maxScanPages < 1 {
		maxScanPages = 1
	}
	if cliutil.IsDogfoodEnv() && maxScanPages > 1 {
		maxScanPages = 1
	}
	res := &fpSweepResult{MaxScanPage: maxScanPages, ScanCapHit: true}
	for page := 0; page < maxScanPages; page++ {
		raw, err := c.Get(ctx, url, q.params(fpPageSize, page*fpPageSize))
		if err != nil {
			return nil, fmt.Errorf("fetching vendors page %d: %w", page+1, err)
		}
		var env fpDiscoEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("parsing vendors page %d: %w", page+1, err)
		}
		res.PagesRead++
		res.Available = env.Data.AvailableCount
		res.Scanned += len(env.Data.Items)
		res.Vendors = append(res.Vendors, env.Data.Items...)
		if len(env.Data.Items) < fpPageSize {
			res.ScanCapHit = false
			break
		}
		if res.Scanned >= env.Data.AvailableCount && env.Data.AvailableCount > 0 {
			res.ScanCapHit = false
			break
		}
	}
	return res, nil
}

// fpAddress is a saved delivery address from the authenticated customer API.
type fpAddress struct {
	ID           float64 `json:"id"`
	City         string  `json:"city"`
	AddressLine1 string  `json:"address_line1"`
	AddressLine2 string  `json:"address_line2"`
	Label        string  `json:"label"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	IsDefault    bool    `json:"is_default"`
}

// Describe returns a short human label without leaking the full street address
// into logs by default.
func (a fpAddress) Describe() string {
	if a.Label != "" {
		return a.Label
	}
	if a.AddressLine1 != "" {
		return a.AddressLine1
	}
	return a.City
}

type fpAddressEnvelope struct {
	Data struct {
		Items []fpAddress `json:"items"`
	} `json:"data"`
}

// fpFetchAddresses returns the caller's saved addresses. Requires auth.
func fpFetchAddresses(ctx context.Context, c *client.Client) ([]fpAddress, error) {
	raw, err := c.Get(ctx, fpAddresses, nil)
	if err != nil {
		return nil, err
	}
	var env fpAddressEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing saved addresses: %w", err)
	}
	return env.Data.Items, nil
}

// fpPickAddress selects the address matching label, else the default, else the
// first. Returns a usable error naming what is available when nothing matches.
func fpPickAddress(addrs []fpAddress, label string) (fpAddress, error) {
	if len(addrs) == 0 {
		return fpAddress{}, fmt.Errorf("no saved addresses found; run 'foodpanda-pp-cli auth login --chrome' from a browser signed in to foodpanda")
	}
	if label != "" {
		want := strings.ToLower(label)
		for _, a := range addrs {
			if strings.ToLower(a.Label) == want || strings.Contains(strings.ToLower(a.AddressLine1), want) {
				return a, nil
			}
		}
		var have []string
		for _, a := range addrs {
			if d := a.Describe(); d != "" {
				have = append(have, d)
			}
		}
		return fpAddress{}, fmt.Errorf("no saved address matching %q (have: %s)", label, strings.Join(have, ", "))
	}
	for _, a := range addrs {
		if a.IsDefault {
			return a, nil
		}
	}
	return addrs[0], nil
}

// fpHaversineKm returns great-circle distance in kilometres.
func fpHaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// fpMatchScore scores how strongly a vendor matches a free-text query.
// foodpanda's search is fuzzy and never returns an empty set, so weak matches
// are indistinguishable from real ones upstream. 100 = name contains the whole
// query; 0 = no token overlap anywhere.
func fpMatchScore(v fpVendor, query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}
	name := strings.ToLower(v.Name)
	if strings.Contains(name, q) {
		return 100
	}
	hay := name + " " + strings.ToLower(v.PrimaryCuisine())
	for _, c := range v.Cuisines {
		hay += " " + strings.ToLower(c.Name)
	}
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return 0
	}
	hits := 0
	for _, t := range tokens {
		if strings.Contains(hay, t) {
			hits++
		}
	}
	return hits * 100 / len(tokens)
}

// fpMatchLabel converts a score into an agent-readable confidence band.
func fpMatchLabel(score int) string {
	switch {
	case score >= 100:
		return "strong"
	case score >= 50:
		return "partial"
	case score > 0:
		return "weak"
	default:
		return "none"
	}
}

// fpSortVendors sorts in place by a named key. Unknown keys leave order intact.
func fpSortVendors(vs []fpVendor, key string) {
	switch key {
	case "fee":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].MinDeliveryFee < vs[j].MinDeliveryFee })
	case "rating":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].Rating > vs[j].Rating })
	case "distance":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].Distance < vs[j].Distance })
	case "time":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].MinDeliveryTime < vs[j].MinDeliveryTime })
	case "min-order":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].MinOrderAmount < vs[j].MinOrderAmount })
	case "points":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].VendorPoints > vs[j].VendorPoints })
	case "name":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].Name < vs[j].Name })
	}
}

// fpTrim caps a slice without panicking on short input. limit <= 0 means all.
func fpTrim[T any](in []T, limit int) []T {
	if limit <= 0 || limit >= len(in) {
		return in
	}
	return in[:limit]
}

// fpRound2 rounds to two decimals for stable money output.
func fpRound2(f float64) float64 { return math.Round(f*100) / 100 }

// fpFeePricing reports how many vendors came back with a real delivery fee.
//
// foodpanda computes delivery fees in a separate Dynamic Pricing Service
// (vendor payloads carry delivery_fee_source: "dps"). Without an active pricing
// session, DPS-priced vendors return minimum_delivery_fee 0, which is
// indistinguishable from genuinely free delivery. Vendors on their own fleet
// still report a real fee, so a handful of non-zero rows does NOT mean pricing
// worked. Callers must surface the ratio rather than printing a bare 0.
func fpFeePricing(vs []fpVendor) (priced, total int, note string) {
	total = len(vs)
	if total == 0 {
		return 0, 0, ""
	}
	for _, v := range vs {
		if v.MinDeliveryFee > 0 {
			priced++
		}
	}
	// Fewer than a fifth priced means DPS did not price this request.
	if priced*5 < total {
		note = fmt.Sprintf("only %d of %d vendors returned a delivery fee; foodpanda prices delivery in a "+
			"separate dynamic-pricing service that stays silent without a session, so a 0 here means "+
			"unpriced rather than free. Run 'foodpanda-pp-cli auth login --chrome' and retry for "+
			"session-priced fees. Minimum order, service fee and VAT are unaffected.", priced, total)
	}
	return priced, total, note
}

// fpPerseusHeaders returns the client-generated tracking identifiers that
// pk.fd-api.com's vendor-detail endpoint requires.
//
// These are NOT credentials — the upstream only checks presence and shape.
// Attach them to the menu host only: sending them to the disco listing makes
// the dynamic-pricing service return a 0 delivery fee for every vendor.
func fpPerseusHeaders() map[string]string {
	const id = "1786146573072.519579439102525090.efuvxeu3"
	return map[string]string{"perseus-client-id": id, "perseus-session-id": id}
}
