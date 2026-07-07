// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package grubhub

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Money is Grubhub's price shape: an integer number of cents plus a currency.
type Money struct {
	Price    int    `json:"price"`
	Currency string `json:"currency"`
}

// Rating captures the restaurant rating fields a card carries.
type Rating struct {
	RatingValue       string  `json:"rating_value"`
	ActualRatingValue float64 `json:"actual_rating_value"`
	RatingCount       float64 `json:"rating_count"`
}

// Offer is one promotional offer attached to a restaurant card.
type Offer struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	OfferType   string `json:"offer_type"`
	Amount      struct {
		Value        float64 `json:"value"`
		Type         string  `json:"type"`
		Currency     string  `json:"currency"`
		OrderMinimum float64 `json:"order_minimum"`
	} `json:"amount"`
}

// ValueCents returns the offer's discount amount in integer cents. Grubhub
// encodes amount.value in cents (observed: 500 == $5.00); rounding rather than
// truncating guards against any fractional encoding a future response might use.
func (o Offer) ValueCents() int { return int(math.Round(o.Amount.Value)) }

// OrderMinimumCents returns the offer's order-minimum threshold in integer cents.
func (o Offer) OrderMinimumCents() int { return int(math.Round(o.Amount.OrderMinimum)) }

// Card is the subset of a Grubhub search result we expose and cache.
type Card struct {
	ID                  string            `json:"restaurant_id"`
	Name                string            `json:"name"`
	Cuisines            []string          `json:"cuisines"`
	DeliveryFee         Money             `json:"delivery_fee"`
	DeliveryMinimum     Money             `json:"delivery_minimum"`
	DeliveryTime        int               `json:"delivery_time_estimate"`
	Distance            string            `json:"distance_from_location"`
	Ratings             Rating            `json:"ratings"`
	PriceRating         float64           `json:"price_rating"`
	Open                bool              `json:"open"`
	CouponsAvailable    bool              `json:"coupons_available"`
	AvailableOffers     []Offer           `json:"available_offers"`
	AvailablePromoCodes []json.RawMessage `json:"available_promo_codes"`
}

// Rating returns the best available numeric rating (0 when unrated).
func (c Card) Rating() float64 {
	if c.Ratings.ActualRatingValue > 0 {
		return c.Ratings.ActualRatingValue
	}
	if v, err := strconv.ParseFloat(c.Ratings.RatingValue, 64); err == nil {
		return v
	}
	return 0
}

// DistanceMiles parses the string distance to a float (0 when unparseable).
func (c Card) DistanceMiles() float64 {
	v, _ := strconv.ParseFloat(c.Distance, 64)
	return v
}

// DealCount returns how many actionable deals the card advertises.
func (c Card) DealCount() int {
	n := len(c.AvailableOffers) + len(c.AvailablePromoCodes)
	if c.CouponsAvailable {
		n++
	}
	return n
}

type searchEnvelope struct {
	SearchResult struct {
		Results []Card `json:"results"`
		Stats   struct {
			TotalResults int `json:"total_results"`
		} `json:"stats"`
	} `json:"search_result"`
}

// ParseSearch extracts restaurant cards and the total-result count from a search
// response body.
func ParseSearch(raw json.RawMessage) ([]Card, int, error) {
	var env searchEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, 0, fmt.Errorf("parsing Grubhub search response: %w", err)
	}
	return env.SearchResult.Results, env.SearchResult.Stats.TotalResults, nil
}

type geocodeEntry struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Locality  string `json:"address_locality"`
	Region    string `json:"address_region"`
	Postal    string `json:"postal_code"`
}

// Coordinates is a resolved address point.
type Coordinates struct {
	Lat      float64
	Lng      float64
	Locality string
	Region   string
	Postal   string
}

// ParseGeocode reads the first coordinate from a /geocode response array.
func ParseGeocode(raw json.RawMessage) (Coordinates, error) {
	var entries []geocodeEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Coordinates{}, fmt.Errorf("parsing Grubhub geocode response: %w", err)
	}
	if len(entries) == 0 {
		return Coordinates{}, fmt.Errorf("no coordinates found for that address")
	}
	e := entries[0]
	lat, err1 := strconv.ParseFloat(e.Latitude, 64)
	lng, err2 := strconv.ParseFloat(e.Longitude, 64)
	if err1 != nil || err2 != nil || (lat == 0 && lng == 0) {
		return Coordinates{}, fmt.Errorf("address did not resolve to usable coordinates")
	}
	return Coordinates{Lat: lat, Lng: lng, Locality: e.Locality, Region: e.Region, Postal: e.Postal}, nil
}

// MenuItem is a single menu entry.
type MenuItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"-"`
	Popular     bool   `json:"popular"`
	ItemCoupon  bool   `json:"item_coupon"`
	Price       struct {
		Amount   int    `json:"amount"`
		Currency string `json:"currency"`
	} `json:"price"`
	// MinPriceVariation holds the starting price for build-your-own items whose
	// flat price.amount is 0 because the total depends on selections.
	MinPriceVariation struct {
		Amount int `json:"amount"`
	} `json:"minimum_price_variation"`
}

// PriceCents returns the item's effective price in cents, falling back to the
// minimum price variation when the flat price is zero (build-your-own items).
func (m MenuItem) PriceCents() int {
	if m.Price.Amount > 0 {
		return m.Price.Amount
	}
	return m.MinPriceVariation.Amount
}

type restaurantEnvelope struct {
	Restaurant struct {
		Name         string `json:"name"`
		MenuCategory []struct {
			Name     string     `json:"name"`
			ItemList []MenuItem `json:"menu_item_list"`
		} `json:"menu_category_list"`
	} `json:"restaurant"`
}

// ParseMenu flattens a restaurant detail response into a name + menu item list.
func ParseMenu(raw json.RawMessage) (name string, items []MenuItem, err error) {
	var env restaurantEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", nil, fmt.Errorf("parsing Grubhub restaurant response: %w", err)
	}
	for _, cat := range env.Restaurant.MenuCategory {
		for _, it := range cat.ItemList {
			it.Category = cat.Name
			items = append(items, it)
		}
	}
	return env.Restaurant.Name, items, nil
}

// ChoiceOption is one selectable modifier option (e.g. a topping).
type ChoiceOption struct {
	Description string `json:"description"`
	Price       struct {
		Amount int `json:"amount"`
	} `json:"price"`
}

// ChoiceCategory is a group of modifier options (e.g. "Bases", "Add-ons").
type ChoiceCategory struct {
	Name    string         `json:"name"`
	Min     int            `json:"min_choice_options"`
	Max     int            `json:"max_choice_options"`
	Options []ChoiceOption `json:"choice_option_list"`
}

// ItemDetail is a single menu item with its modifier categories.
type ItemDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"menu_category_name"`
	Price       struct {
		Amount int `json:"amount"`
	} `json:"price"`
	ChoiceCategories []ChoiceCategory `json:"choice_category_list"`
}

// ParseItem reads a single menu item detail response (top-level object).
func ParseItem(raw json.RawMessage) (ItemDetail, error) {
	var item ItemDetail
	if err := json.Unmarshal(raw, &item); err != nil {
		return ItemDetail{}, fmt.Errorf("parsing Grubhub menu item response: %w", err)
	}
	return item, nil
}

// Dollars renders an integer cents value as a $-prefixed string.
func Dollars(cents int) string {
	return "$" + strconv.FormatFloat(float64(cents)/100.0, 'f', 2, 64)
}
