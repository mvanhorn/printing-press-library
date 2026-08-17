// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"encoding/json"
	"strings"
)

// ExtractFavoriteIDs pulls listing ids from fav_board / project attachment JSON.
// Supports:
//   - {"attachments":[{"value":"..."}]}
//   - [{"value":"..."}]
//   - {"value":"..."}
//   - raw string id
func ExtractFavoriteIDs(data []byte) []string {
	out := make([]string, 0)
	if len(data) == 0 {
		return out
	}
	// envelope with attachments
	var env struct {
		Attachments []struct {
			Value string `json:"value"`
			ID    string `json:"id"`
			ID2   string `json:"_id"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(data, &env); err == nil && len(env.Attachments) > 0 {
		for _, a := range env.Attachments {
			if a.Value != "" {
				out = append(out, a.Value)
			}
		}
		return uniqueStrings(out)
	}
	// array of attachments or ids
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		for _, item := range arr {
			var obj map[string]any
			if json.Unmarshal(item, &obj) == nil {
				if v, ok := obj["value"].(string); ok && v != "" {
					out = append(out, v)
					continue
				}
				if v := firstString(obj, "id", "_id", "listing_id"); v != "" {
					out = append(out, v)
					continue
				}
			}
			var s string
			if json.Unmarshal(item, &s) == nil && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return uniqueStrings(out)
	}
	// single object
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		if v, ok := obj["value"].(string); ok && v != "" {
			return []string{v}
		}
		// project details may nest favorites
		if favs, ok := obj["favorites"].([]any); ok {
			for _, f := range favs {
				switch t := f.(type) {
				case string:
					if t != "" {
						out = append(out, t)
					}
				case map[string]any:
					if v := firstString(t, "value", "id", "_id"); v != "" {
						out = append(out, v)
					}
				}
			}
			return uniqueStrings(out)
		}
	}
	// bare string
	var s string
	if err := json.Unmarshal(data, &s); err == nil && strings.TrimSpace(s) != "" {
		return []string{strings.TrimSpace(s)}
	}
	return out
}

// JoinFavorites maps favorite ids to listings (missing ids still emit a stub row).
func JoinFavorites(favIDs []string, listings []Listing) []Listing {
	byID := map[string]Listing{}
	for _, l := range listings {
		if l.ID != "" {
			byID[l.ID] = l
		}
	}
	out := make([]Listing, 0, len(favIDs))
	for _, id := range favIDs {
		if l, ok := byID[id]; ok {
			out = append(out, l)
			continue
		}
		out = append(out, Listing{ID: id})
	}
	return out
}

// SortListings sorts by price|capacity|rating.
func SortListings(in []Listing, sortBy string) []Listing {
	out := append([]Listing(nil), in...)
	key := strings.ToLower(strings.TrimSpace(sortBy))
	switch key {
	case "capacity", "guests":
		sortListingsBy(out, func(a, b Listing) bool {
			if a.Guests != b.Guests {
				return a.Guests > b.Guests
			}
			return a.ID < b.ID
		})
	case "rating", "stars", "review":
		sortListingsBy(out, func(a, b Listing) bool {
			if a.ReviewStars != b.ReviewStars {
				return a.ReviewStars > b.ReviewStars
			}
			if a.ReviewCount != b.ReviewCount {
				return a.ReviewCount > b.ReviewCount
			}
			return a.ID < b.ID
		})
	default: // price
		sortListingsBy(out, func(a, b Listing) bool {
			// zero prices last
			if a.PriceHourly == 0 && b.PriceHourly != 0 {
				return false
			}
			if b.PriceHourly == 0 && a.PriceHourly != 0 {
				return true
			}
			if a.PriceHourly != b.PriceHourly {
				return a.PriceHourly < b.PriceHourly
			}
			return a.ID < b.ID
		})
	}
	return out
}

func sortListingsBy(in []Listing, less func(a, b Listing) bool) {
	// insertion sort is fine for shortlist sizes
	for i := 1; i < len(in); i++ {
		j := i
		for j > 0 && less(in[j], in[j-1]) {
			in[j], in[j-1] = in[j-1], in[j]
			j--
		}
	}
}
