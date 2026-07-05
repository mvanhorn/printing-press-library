// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// SearchParams describes a stays search. Only Location is required.
type SearchParams struct {
	Location    string
	Checkin     string // YYYY-MM-DD
	Checkout    string // YYYY-MM-DD
	Adults      int
	Children    int
	Infants     int
	Pets        int
	PriceMin    int
	PriceMax    int
	RoomTypes   []string // e.g. "Entire home/apt", "Private room"
	MinBedrooms int
	ItemsPerGrid int
	Cursor      string // pageCursors entry for pagination
}

// rawParam is one Airbnb search filter: a name and its string values.
type rawParam struct {
	FilterName   string   `json:"filterName"`
	FilterValues []string `json:"filterValues"`
}

func (p SearchParams) rawParams() []rawParam {
	rp := []rawParam{
		{FilterName: "query", FilterValues: []string{p.Location}},
		{FilterName: "refinementPaths", FilterValues: []string{"/homes"}},
		{FilterName: "search_type", FilterValues: []string{"filter_change"}},
	}
	add := func(name, val string) {
		if val != "" {
			rp = append(rp, rawParam{FilterName: name, FilterValues: []string{val}})
		}
	}
	if p.Adults > 0 {
		add("adults", strconv.Itoa(p.Adults))
	}
	if p.Children > 0 {
		add("children", strconv.Itoa(p.Children))
	}
	if p.Infants > 0 {
		add("infants", strconv.Itoa(p.Infants))
	}
	if p.Pets > 0 {
		add("pets", strconv.Itoa(p.Pets))
	}
	add("checkin", p.Checkin)
	add("checkout", p.Checkout)
	if p.PriceMin > 0 {
		add("priceMin", strconv.Itoa(p.PriceMin))
	}
	if p.PriceMax > 0 {
		add("priceMax", strconv.Itoa(p.PriceMax))
	}
	if p.MinBedrooms > 0 {
		add("minBedrooms", strconv.Itoa(p.MinBedrooms))
	}
	for _, rt := range p.RoomTypes {
		rp = append(rp, rawParam{FilterName: "room_types", FilterValues: []string{rt}})
	}
	grid := p.ItemsPerGrid
	if grid <= 0 {
		grid = 18
	}
	add("itemsPerGrid", strconv.Itoa(grid))
	return rp
}

// SearchResult is the flattened shape used for tables and CSV.
type SearchResult struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Title     string  `json:"title"`
	Price     string  `json:"price"`
	Rating    string  `json:"rating"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	URL       string  `json:"url"`
}

// Search runs a stays search and returns both the flattened results and the
// raw searchResults JSON (for --json / --select).
func (c *Client) Search(p SearchParams) ([]SearchResult, json.RawMessage, error) {
	if p.Location == "" {
		return nil, nil, fmt.Errorf("search requires a location")
	}
	vars := map[string]any{
		"staysSearchRequest": map[string]any{
			"requestedPageType": "STAYS_SEARCH",
			"metadataOnly":      false,
			"treatmentFlags":    []string{},
			"rawParams":         p.rawParams(),
			"maxMapItems":       9999,
		},
		"isLeanTreatment":  false,
		"aiSearchEnabled":  false,
	}
	if p.Cursor != "" {
		vars["staysSearchRequest"].(map[string]any)["cursor"] = p.Cursor
	}
	data, err := c.Query("StaysSearch", vars)
	if err != nil {
		return nil, nil, err
	}
	raw := rawAtPath(data, "presentation", "staysSearch", "results", "searchResults")
	results := parseSearchResults(raw, c.baseURL)
	return results, raw, nil
}

func parseSearchResults(raw json.RawMessage, baseURL string) []SearchResult {
	var arr []struct {
		Title       string `json:"title"`
		NameLocalized struct {
			Localized string `json:"localizedStringWithTranslationPreference"`
		} `json:"nameLocalized"`
		AvgRatingLocalized string `json:"avgRatingLocalized"`
		StructuredDisplayPrice struct {
			PrimaryLine struct {
				AccessibilityLabel string `json:"accessibilityLabel"`
				DiscountedPrice    string `json:"discountedPrice"`
				OriginalPrice      string `json:"originalPrice"`
				Price              string `json:"price"`
			} `json:"primaryLine"`
		} `json:"structuredDisplayPrice"`
		DemandStayListing struct {
			ID       string `json:"id"`
			Location struct {
				Coordinate struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"coordinate"`
			} `json:"location"`
		} `json:"demandStayListing"`
	}
	if json.Unmarshal(raw, &arr) != nil {
		return nil
	}
	out := make([]SearchResult, 0, len(arr))
	for _, r := range arr {
		id := NumericID(r.DemandStayListing.ID)
		price := r.StructuredDisplayPrice.PrimaryLine.AccessibilityLabel
		if price == "" {
			price = firstNonEmpty(r.StructuredDisplayPrice.PrimaryLine.DiscountedPrice, r.StructuredDisplayPrice.PrimaryLine.Price, r.StructuredDisplayPrice.PrimaryLine.OriginalPrice)
		}
		out = append(out, SearchResult{
			ID:        id,
			Name:      r.NameLocalized.Localized,
			Title:     r.Title,
			Price:     price,
			Rating:    r.AvgRatingLocalized,
			Latitude:  r.DemandStayListing.Location.Coordinate.Latitude,
			Longitude: r.DemandStayListing.Location.Coordinate.Longitude,
			URL:       fmt.Sprintf("%s/rooms/%s", baseURL, id),
		})
	}
	return out
}

// Autocomplete returns location suggestions for a free-text query.
func (c *Client) Autocomplete(query string) (json.RawMessage, error) {
	vars := map[string]any{
		"autoSuggestionsRequest": map[string]any{
			"rawParams": []rawParam{
				{FilterName: "userInput", FilterValues: []string{query}},
			},
			"source":         "P1_HOMEPAGE",
			"treatmentFlags": []string{},
		},
	}
	return c.Query("AutoSuggestionsQuery", vars)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
