// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import (
	"encoding/json"
	"strconv"
)

// Listing is a parsed AutoTempest search result, normalized into clean typed
// fields for persistence and display. PriceCents/Mileage are -1 when unknown
// and Year is 0 when unknown (see the Parse* helpers).
type Listing struct {
	ID           string          `json:"id"`
	VIN          string          `json:"vin"`
	Title        string          `json:"title"`
	Make         string          `json:"make"`
	Model        string          `json:"model"`
	Year         int             `json:"year"`
	Trim         string          `json:"trim"`
	PriceCents   int64           `json:"price_cents"`
	Mileage      int64           `json:"mileage"`
	Location     string          `json:"location"`
	Zip          string          `json:"zip"`
	Country      string          `json:"country"`
	Distance     float64         `json:"distance"`
	DealerName   string          `json:"dealer_name"`
	SellerType   string          `json:"seller_type"`
	Source       string          `json:"source"`   // queried source code (te/hem/cs/...)
	Sitecode     string          `json:"sitecode"` // backend feed code (ll, ...)
	VehicleTitle string          `json:"vehicle_title"`
	ListingType  string          `json:"listing_type"`
	CurrentBid   int64           `json:"current_bid_cents"`
	Bids         int64           `json:"bids"`
	URL          string          `json:"url"`
	Img          string          `json:"img"`
	Raw          json.RawMessage `json:"-"`
}

// rawListing mirrors the on-the-wire listing object. Most numeric fields arrive
// as strings; price/mileage/year may also be empty. Use json.Number for the
// genuinely numeric fields (distance, bids) so we tolerate either shape.
type rawListing struct {
	ID           string          `json:"id"`
	VIN          string          `json:"vin"`
	Title        string          `json:"title"`
	Make         string          `json:"make"`
	Model        string          `json:"model"`
	BackendModel string          `json:"backendModel"`
	Year         string          `json:"year"`
	Trim         string          `json:"trim"`
	Price        string          `json:"price"`
	Mileage      string          `json:"mileage"`
	Location     string          `json:"location"`
	LocationCode string          `json:"locationCode"`
	CountryCode  string          `json:"countryCode"`
	Distance     json.Number     `json:"distance"`
	DealerName   string          `json:"dealerName"`
	SellerType   string          `json:"sellerType"`
	URL          string          `json:"url"`
	Img          string          `json:"img"`
	Sitecode     string          `json:"sitecode"`
	VehicleTitle string          `json:"vehicleTitle"`
	ListingType  string          `json:"listingType"`
	CurrentBid   json.RawMessage `json:"currentBid"`
	Bids         json.RawMessage `json:"bids"`
}

// ParseListing decodes one raw listing JSON object into a typed Listing, tagging
// it with the queried source code. Returns ok=false when the object has no
// usable id (the stable persistence key).
func ParseListing(raw json.RawMessage, source string) (Listing, bool) {
	var r rawListing
	if err := json.Unmarshal(raw, &r); err != nil {
		return Listing{}, false
	}
	if r.ID == "" {
		return Listing{}, false
	}
	model := r.Model
	if model == "" {
		model = r.BackendModel
	}
	var dist float64
	if r.Distance != "" {
		dist, _ = r.Distance.Float64()
	}
	l := Listing{
		ID:           r.ID,
		VIN:          r.VIN,
		Title:        r.Title,
		Make:         r.Make,
		Model:        model,
		Year:         ParseYear(r.Year),
		Trim:         r.Trim,
		PriceCents:   ParsePriceCents(r.Price),
		Mileage:      ParseMileage(r.Mileage),
		Location:     r.Location,
		Zip:          r.LocationCode,
		Country:      r.CountryCode,
		Distance:     dist,
		DealerName:   r.DealerName,
		SellerType:   r.SellerType,
		Source:       source,
		Sitecode:     r.Sitecode,
		VehicleTitle: r.VehicleTitle,
		ListingType:  r.ListingType,
		CurrentBid:   parseLooseCurrentBid(r.CurrentBid),
		Bids:         parseLooseInt(r.Bids),
		URL:          r.URL,
		Img:          r.Img,
		Raw:          raw,
	}
	return l, true
}

// parseLooseCurrentBid handles currentBid arriving as a string ("$1,200"),
// a number, or null. Returns -1 when absent/unknown.
func parseLooseCurrentBid(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return -1
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return ParsePriceCents(s)
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		if f, err := n.Float64(); err == nil {
			return int64(f*100 + 0.5)
		}
	}
	return -1
}

// parseLooseInt handles a field arriving as a number, numeric string, or null.
// Returns -1 when absent/unknown.
func parseLooseInt(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return -1
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
	}
	return -1
}
