// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: typed structs for parsed cards, details, and enumerations.

package motohunt

// Listing is a parsed search-result card.
type Listing struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Price      string   `json:"price,omitempty"`
	Mileage    string   `json:"mileage,omitempty"`
	Condition  string   `json:"condition,omitempty"`
	DealRating string   `json:"deal_rating,omitempty"`
	Badges     []string `json:"badges,omitempty"`
	Location   string   `json:"location,omitempty"`
	Dealer     string   `json:"dealer,omitempty"`
	Image      string   `json:"image,omitempty"`
	URL        string   `json:"url,omitempty"`
}

// ListingDetail is the full /l/{id} page including the price-research block.
type ListingDetail struct {
	ID                string   `json:"id"`
	Title             string   `json:"title,omitempty"`
	Subtitle          string   `json:"subtitle,omitempty"`
	CertifiedPreOwned bool     `json:"certified_pre_owned"`
	Location          string   `json:"location,omitempty"`
	Condition         string   `json:"condition,omitempty"`
	Dealer            string   `json:"dealer,omitempty"`
	Price             string   `json:"price,omitempty"`
	Mileage           string   `json:"mileage,omitempty"`
	Color             string   `json:"color,omitempty"`
	Age               string   `json:"age,omitempty"`
	StockNumber       string   `json:"stock_number,omitempty"`
	VIN               string   `json:"vin,omitempty"`
	Description       string   `json:"description,omitempty"`
	BaseMSRP          string   `json:"base_msrp,omitempty"`
	ALP               string   `json:"alp,omitempty"`
	DealRating        string   `json:"deal_rating,omitempty"`
	Images            []string `json:"images,omitempty"`
	URL               string   `json:"url,omitempty"`
}

// Make is one entry from the make dropdown / browse block.
type Make struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Model is one entry from the model-selector cascade.
type Model struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Section string `json:"section,omitempty"` // style/category heading the model sits under
}
