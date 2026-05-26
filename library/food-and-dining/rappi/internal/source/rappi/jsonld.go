// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
package rappi

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Restaurant captures the schema.org Restaurant block embedded in
// rappi.com.mx restaurant detail pages. Field shape mirrors the JSON-LD
// content; not every restaurant exposes every field.
type Restaurant struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	URL           string             `json:"url,omitempty"`
	Image         string             `json:"image,omitempty"`
	Logo          string             `json:"logo,omitempty"`
	ServesCuisine []string           `json:"serves_cuisine,omitempty"`
	AddressStreet string             `json:"address_street,omitempty"`
	OpeningHours  []OpeningHoursSpec `json:"opening_hours,omitempty"`
	Latitude      float64            `json:"latitude,omitempty"`
	Longitude     float64            `json:"longitude,omitempty"`
	RatingValue   float64            `json:"rating,omitempty"`
	RatingCount   int                `json:"review_count,omitempty"`
	RatingBest    float64            `json:"rating_best,omitempty"`
	Description   string             `json:"description,omitempty"`
	City          string             `json:"city,omitempty"`
	Category      string             `json:"category,omitempty"`
	Neighborhood  string             `json:"neighborhood,omitempty"`
}

// OpeningHoursSpec mirrors a single schema.org OpeningHoursSpecification entry.
type OpeningHoursSpec struct {
	DayOfWeek string `json:"day_of_week"`
	Opens     string `json:"opens,omitempty"`
	Closes    string `json:"closes,omitempty"`
}

// Store captures the lightweight schema.org reference for a Rappi store
// listing. Detail pages don't always emit a full Store @type, so this is
// the canonical store-record shape used by the local catalog.
type Store struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	URL       string  `json:"url,omitempty"`
	StoreType string  `json:"store_type,omitempty"`
	City      string  `json:"city,omitempty"`
	Address   string  `json:"address,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Image     string  `json:"image,omitempty"`
}

// RestaurantListItem is the row shape extracted from an ItemList JSON-LD
// block on restaurant index pages. Each restaurant detail page provides a
// richer Restaurant record (see Restaurant).
type RestaurantListItem struct {
	Position      int     `json:"position"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Image         string  `json:"image,omitempty"`
	ServesCuisine string  `json:"serves_cuisine,omitempty"`
	RatingValue   float64 `json:"rating,omitempty"`
	RatingCount   int     `json:"review_count,omitempty"`
	ID            string  `json:"id,omitempty"`
	City          string  `json:"city,omitempty"`
	Category      string  `json:"category,omitempty"`
}

var jsonLDBlockRe = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)

// idSlugRe captures the numeric ID prefix from a Rappi URL slug
// (e.g. "10000295-el-farolito" -> "10000295").
var idSlugRe = regexp.MustCompile(`/(?:restaurantes|tiendas)/(?:delivery/)?([0-9]+)-([a-zA-Z0-9-]+)`)

// ExtractJSONLDBlocks returns the raw JSON of every <script type="application/ld+json">
// block in the HTML. Each block is independently parseable JSON.
func ExtractJSONLDBlocks(html []byte) []json.RawMessage {
	matches := jsonLDBlockRe.FindAllSubmatch(html, -1)
	out := make([]json.RawMessage, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			out = append(out, json.RawMessage(strings.TrimSpace(string(m[1]))))
		}
	}
	return out
}

// ParseRestaurant walks JSON-LD blocks looking for a Restaurant @type and
// returns the populated Restaurant struct. Returns nil when no Restaurant
// block is found (e.g. when called on a list page).
func ParseRestaurant(html []byte) *Restaurant {
	blocks := ExtractJSONLDBlocks(html)
	for _, b := range blocks {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if asString(m["@type"]) != "Restaurant" {
			continue
		}
		r := Restaurant{
			Name:          asString(m["name"]),
			URL:           asString(m["url"]),
			Image:         asString(m["image"]),
			Logo:          asString(m["logo"]),
			Description:   asString(m["description"]),
			ServesCuisine: asStringSlice(m["servesCuisine"]),
		}
		if id := asString(m["@id"]); id != "" {
			r.ID = idFromURL(id)
		}
		if r.ID == "" {
			r.ID = idFromURL(r.URL)
		}
		if addr, ok := m["address"].(map[string]any); ok {
			r.AddressStreet = asString(addr["streetAddress"])
		}
		if geo, ok := m["geo"].(map[string]any); ok {
			r.Latitude = asFloat(geo["latitude"])
			r.Longitude = asFloat(geo["longitude"])
		}
		if rating, ok := m["aggregateRating"].(map[string]any); ok {
			r.RatingValue = asFloat(rating["ratingValue"])
			r.RatingCount = asInt(rating["ratingCount"])
			r.RatingBest = asFloat(rating["bestRating"])
		}
		if hours, ok := m["openingHoursSpecification"].([]any); ok {
			for _, hRaw := range hours {
				if h, ok := hRaw.(map[string]any); ok {
					r.OpeningHours = append(r.OpeningHours, OpeningHoursSpec{
						DayOfWeek: lastPathSegment(asString(h["dayOfWeek"])),
						Opens:     asString(h["opens"]),
						Closes:    asString(h["closes"]),
					})
				}
			}
		}
		r.Neighborhood = extractNeighborhood(r.AddressStreet)
		return &r
	}
	return nil
}

// ParseRestaurantList scans JSON-LD ItemList blocks and returns the
// extracted restaurant rows. Rappi list pages emit two ItemList blocks:
// one with just position+name+url, the other with full restaurant
// entries (image, cuisine, rating). We merge the richer block whenever
// available and fall back to the lean one for restaurants only present
// in the leaner list.
func ParseRestaurantList(html []byte, city, category string) []RestaurantListItem {
	blocks := ExtractJSONLDBlocks(html)
	byURL := map[string]RestaurantListItem{}
	for _, b := range blocks {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if asString(m["@type"]) != "ItemList" {
			continue
		}
		items, ok := m["itemListElement"].([]any)
		if !ok {
			continue
		}
		for _, raw := range items {
			it, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			pos := asInt(it["position"])
			name := asString(it["name"])
			url := asString(it["url"])
			// Some blocks wrap the data in `item`.
			if inner, ok := it["item"].(map[string]any); ok {
				if asString(inner["@type"]) == "Restaurant" {
					row := RestaurantListItem{
						Position:      pos,
						Name:          asString(inner["name"]),
						URL:           asString(inner["url"]),
						Image:         asString(inner["image"]),
						ServesCuisine: asString(inner["servesCuisine"]),
						City:          city,
						Category:      category,
					}
					if r, ok := inner["aggregateRating"].(map[string]any); ok {
						row.RatingValue = asFloat(r["ratingValue"])
						row.RatingCount = asInt(r["reviewCount"])
					}
					row.ID = idFromURL(row.URL)
					if row.URL != "" {
						byURL[row.URL] = row
					}
					continue
				}
			}
			if url == "" {
				continue
			}
			row, exists := byURL[url]
			if !exists {
				row = RestaurantListItem{
					Position: pos,
					Name:     name,
					URL:      url,
					City:     city,
					Category: category,
				}
				row.ID = idFromURL(url)
			}
			byURL[url] = row
		}
	}
	out := make([]RestaurantListItem, 0, len(byURL))
	for _, v := range byURL {
		out = append(out, v)
	}
	return out
}

// ParseStoreList scans JSON-LD ItemList blocks on a store listing page
// (e.g. /tiendas/tipo/market) and returns the position+name+url store
// rows. Store list blocks are leaner than restaurant blocks — they don't
// have a richer wrapping ItemList that includes ratings or geo.
func ParseStoreList(html []byte, storeType, city string) []Store {
	blocks := ExtractJSONLDBlocks(html)
	out := []Store{}
	seen := map[string]bool{}
	for _, b := range blocks {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if asString(m["@type"]) != "ItemList" {
			continue
		}
		items, ok := m["itemListElement"].([]any)
		if !ok {
			continue
		}
		for _, raw := range items {
			it, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			url := asString(it["url"])
			name := asString(it["name"])
			if url == "" {
				continue
			}
			if seen[url] {
				continue
			}
			seen[url] = true
			id := idFromURL(url)
			out = append(out, Store{
				ID:        id,
				Name:      name,
				URL:       url,
				StoreType: storeType,
				City:      city,
			})
		}
	}
	return out
}

// ParseStore walks JSON-LD blocks looking for a Store detail block.
func ParseStore(html []byte) *Store {
	// PATCH: Parse Store JSON-LD detail blocks so adjacency can use real geo.
	blocks := ExtractJSONLDBlocks(html)
	for _, b := range blocks {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if asString(m["@type"]) != "Store" {
			continue
		}
		s := Store{
			Name:  asString(m["name"]),
			URL:   asString(m["url"]),
			Image: asString(m["image"]),
		}
		if id := asString(m["@id"]); id != "" {
			s.ID = idFromURL(id)
		}
		if s.ID == "" {
			s.ID = idFromURL(s.URL)
		}
		if addr, ok := m["address"].(map[string]any); ok {
			s.Address = asString(addr["streetAddress"])
		}
		if geo, ok := m["geo"].(map[string]any); ok {
			s.Latitude = asFloat(geo["latitude"])
			s.Longitude = asFloat(geo["longitude"])
		}
		return &s
	}
	return nil
}

// Helpers ---------------------------------------------------------------

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		var f float64
		_ = json.Unmarshal([]byte(x), &f)
		return f
	}
	return 0
}

func asInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case string:
		var i int
		_ = json.Unmarshal([]byte(x), &i)
		return i
	}
	return 0
}

func asStringSlice(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

func idFromURL(url string) string {
	m := idSlugRe.FindStringSubmatch(url)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func lastPathSegment(url string) string {
	if url == "" {
		return ""
	}
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i+1:]
	}
	return url
}

var neighborhoodRe = regexp.MustCompile(`(?i)\bcol\.\s+([^,.]+)`)

// extractNeighborhood pulls a best-effort neighborhood name from a Mexican
// address string. Rappi addresses commonly look like
// "ALTATA No. 19 LOCALES A,D,E,F,G,H. COL. HIPODROMO CONDESA, CUAUHTEMOC."
// The "COL. <name>" or "Col. <name>" segment is the neighborhood; we
// extract whatever follows COL. up to the next comma or period. The
// matcher requires the "." after COL so a stray sentence like
// "col marker" doesn't trigger.
// Returns "" when no neighborhood marker is found.
func extractNeighborhood(addr string) string {
	if addr == "" {
		return ""
	}
	m := neighborhoodRe.FindStringSubmatch(addr)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
