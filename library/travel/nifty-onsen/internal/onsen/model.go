// Package onsen implements the public, read-only Nifty Onsen website surface.
package onsen

import "time"

const Origin = "https://onsen.nifty.com"
const SchemaVersion = "1"

// Nullable facts stay unknown until explicit source evidence exists.
type Facility struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Area            *string  `json:"area"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	DistanceKM      *float64 `json:"distance_km,omitempty"`
	Rating          *float64 `json:"rating"`
	ReviewCount     *int     `json:"review_count"`
	Hours           *string  `json:"hours"`
	Admission       *string  `json:"admission"`
	MinPriceJPY     *int     `json:"min_price_jpy"`
	PriceBasis      string   `json:"price_basis"`
	DayUse          *bool    `json:"day_use"`
	Stay            *bool    `json:"stay"`
	CouponAvailable *bool    `json:"coupon_available"`
	SourceLabels    []string `json:"source_labels"`
}
type Claim struct {
	State    string   `json:"state"`
	Evidence []string `json:"evidence"`
}
type Detail struct {
	Facility
	ClosureDays         *string           `json:"closure_days"`
	Address             *string           `json:"address"`
	Access              *string           `json:"access"`
	Parking             *string           `json:"parking"`
	Phone               *string           `json:"phone"`
	OfficialURL         *string           `json:"official_url"`
	Facilities          map[string]string `json:"facilities"`
	BathKind            []string          `json:"bath_kind"`
	NaturalHotSpring    Claim             `json:"natural_hot_spring"`
	PrivateRentableBath Claim             `json:"private_rentable_bath"`
	PrivateRoom         Claim             `json:"private_room"`
	PrivateBathEvidence []string          `json:"private_bath_evidence"`
	Policies            map[string]Claim  `json:"policies"`
	ReservableInventory *bool             `json:"reservable_inventory"`
}
type Coupon struct {
	ID                 string   `json:"id"`
	Title              *string  `json:"title"`
	URL                string   `json:"url"`
	PriceText          *string  `json:"price_text"`
	ValidityText       *string  `json:"validity_text"`
	ValidFrom          *string  `json:"valid_from"`
	ValidUntil         *string  `json:"valid_until"`
	Membership         string   `json:"membership"`
	AppOnly            *bool    `json:"app_only"`
	Conditions         []string `json:"conditions"`
	Details            *string  `json:"details"`
	SourceText         string   `json:"source_text,omitempty"`
	SourceTextComplete bool     `json:"source_text_complete"`
	Redeemability      string   `json:"redeemability"`
}
type Provenance struct {
	Freshness
	URL      string   `json:"url"`
	Requests int      `json:"requests"`
	Warnings []string `json:"warnings"`
}
type SearchData struct {
	Items   []Facility `json:"items"`
	NextURL *string    `json:"next_url"`
	Total   *int       `json:"source_total"`
	Page    int        `json:"page"`
}
type CouponData struct {
	FacilityID string   `json:"facility_id"`
	URL        string   `json:"url"`
	Coupons    []Coupon `json:"coupons"`
}
type Freshness struct {
	FetchedAt  string `json:"fetched_at"`
	AgeSeconds int64  `json:"age_seconds"`
	Cache      string `json:"cache"`
	Stale      bool   `json:"stale"`
	Timezone   string `json:"timezone"`
}
type Coverage struct {
	Source      string `json:"source"`
	URL         string `json:"url"`
	Scope       string `json:"scope"`
	Exhaustive  bool   `json:"exhaustive"`
	SourceCount int    `json:"source_count"`
	Returned    int    `json:"returned"`
	Note        string `json:"note"`
}
type Envelope struct {
	SchemaVersion string    `json:"schema_version"`
	Items         any       `json:"items"`
	Freshness     Freshness `json:"freshness"`
	Coverage      Coverage  `json:"coverage"`
	NextPage      *int      `json:"next_page"`
	SourceTotal   *int      `json:"source_total"`
	Warnings      []string  `json:"warnings"`
}

func ptr[T any](v T) *T { return &v }
func strptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func unknown() Claim { return Claim{State: "unknown", Evidence: []string{}} }
func freshness(at time.Time, cache string, ttl time.Duration) Freshness {
	age := int64(time.Since(at).Seconds())
	if age < 0 {
		age = 0
	}
	return Freshness{at.In(time.FixedZone("JST", 9*3600)).Format(time.RFC3339), age, cache, time.Since(at) > ttl, "Asia/Tokyo"}
}
