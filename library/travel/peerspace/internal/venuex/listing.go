// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

// Package venuex holds pure listing parse/band/score/gap helpers for novel
// peerspace-pp-cli commands. No cobra, no SQLite I/O — only data transforms.
package venuex

import (
	"encoding/json"
	"math"
	"strings"
)

// Listing is a normalized venue/search-hit or full detail row.
type Listing struct {
	ID           string          `json:"id"`
	Title        string          `json:"title,omitempty"`
	City         string          `json:"city,omitempty"`
	Neighborhood string          `json:"neighborhood,omitempty"`
	Country      string          `json:"country,omitempty"`
	State        string          `json:"state,omitempty"`
	Guests       int             `json:"guests,omitempty"`
	SpaceType    string          `json:"space_type,omitempty"`
	Description  string          `json:"description,omitempty"` // About / long host copy
	About        string          `json:"about,omitempty"`       // alias of description for export clarity
	Rules        string          `json:"rules,omitempty"`       // host rules block
	Parking      string          `json:"parking,omitempty"`     // parking_info.description
	ParkingAvail *bool           `json:"parking_available,omitempty"`
	Cleaning     string          `json:"cleaning,omitempty"` // cleaning_info.description
	Cancellation string          `json:"cancellation,omitempty"`
	Included     string          `json:"included,omitempty"` // derived from amenities/included copy when present
	SpaceID      string          `json:"space_id,omitempty"` // parentSpaceId for calendar APIs
	Currency     string          `json:"currency,omitempty"`
	Sqft         int             `json:"sqft,omitempty"`
	PriceHourly  float64         `json:"price_hourly,omitempty"`
	InstantBook  bool            `json:"instant_book,omitempty"`
	ReviewStars  float64         `json:"review_stars,omitempty"`
	ReviewCount  int             `json:"review_count,omitempty"`
	HostID       string          `json:"host_id,omitempty"`
	HostName     string          `json:"host_name,omitempty"`
	Amenities    []string        `json:"amenities,omitempty"`
	FormatFit    string          `json:"format_fit,omitempty"` // talk|wellness|fb|production|mixed
	Lat          float64         `json:"lat,omitempty"`
	Lon          float64         `json:"lon,omitempty"`
	Hydrated     bool            `json:"hydrated,omitempty"` // true when from GET /v1/listings/{id}
	Raw          json.RawMessage `json:"-"`
}

// ExpandResourceData turns a resources.data blob into zero or more listings.
// Full search responses with hits.hits are expanded into one listing per hit.
func ExpandResourceData(resourceID string, data []byte) []Listing {
	out := make([]Listing, 0)
	if len(data) == 0 {
		return out
	}
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		return out
	}
	// Elasticsearch-shaped search envelope.
	if hitsObj, ok := top["hits"].(map[string]any); ok {
		if arr, ok := hitsObj["hits"].([]any); ok && len(arr) > 0 {
			for _, h := range arr {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				// Prefer _source when present; otherwise treat hit as the listing.
				src := hm
				if nested, ok := hm["_source"].(map[string]any); ok {
					src = nested
				}
				raw, _ := json.Marshal(src)
				l, ok := ParseListing(raw)
				if !ok {
					continue
				}
				if l.ID == "" {
					if id, _ := hm["_id"].(string); id != "" {
						l.ID = id
					}
				}
				out = append(out, l)
			}
			return out
		}
	}
	// Array of listings.
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		for _, item := range arr {
			if l, ok := ParseListing(item); ok {
				out = append(out, l)
			}
		}
		return out
	}
	// Single listing object.
	if l, ok := ParseListing(data); ok {
		if l.ID == "" {
			l.ID = resourceID
		}
		out = append(out, l)
	}
	return out
}

// ParseListing flexibly extracts fields from Peerspace listing / search-hit JSON.
func ParseListing(data []byte) (Listing, bool) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return Listing{}, false
	}
	l := Listing{Raw: append(json.RawMessage(nil), data...)}
	l.ID = firstString(m, "id", "_id")
	l.Title = firstString(m, "title", "name")
	l.City = firstString(m, "city", "metro")
	l.Neighborhood = firstString(m, "neighborhood")
	l.Country = firstString(m, "country", "country_long")
	l.State = firstString(m, "state", "state_long")
	l.SpaceType = firstString(m, "space_type_tag", "space_type", "room_type")
	l.Description = firstString(m, "description", "about")
	l.About = l.Description
	l.Rules = firstString(m, "rules", "host_rules", "space_rules")
	l.SpaceID = firstString(m, "parentSpaceId", "parent_space_id", "space_id", "spaceId")
	l.HostID = firstString(m, "host_id", "ownerId", "owner_id")
	if l.HostID == "" {
		if hr, ok := m["host_responsiveness"].(map[string]any); ok {
			l.HostID = firstString(hr, "host_id")
		}
	}
	first := firstString(m, "ownerFirstName", "owner_first_name")
	last := firstString(m, "ownerLastName", "owner_last_name")
	l.HostName = strings.TrimSpace(strings.Join([]string{first, last}, " "))
	l.Guests = firstInt(m, "number_guests", "attendees_max", "guests", "capacity", "max_guests")
	l.Sqft = firstInt(m, "sqft", "square_feet")
	l.ReviewStars = firstFloat(m, "space_review_stars", "review_stars", "rating", "stars")
	l.ReviewCount = firstInt(m, "space_review_count", "review_count", "reviews")
	l.InstantBook = firstBool(m, "is_instant_book_active", "instant_book", "instantBook")
	l.PriceHourly = extractPrice(m)
	l.Currency = extractCurrency(m)
	l.Amenities = extractAmenities(m)
	// Full detail page blocks (GET /v1/listings/{id})
	if pi, ok := m["parking_info"].(map[string]any); ok {
		l.Parking = firstString(pi, "description")
		if v, ok := pi["is_available"].(bool); ok {
			l.ParkingAvail = &v
		}
	}
	if ci, ok := m["cleaning_info"].(map[string]any); ok {
		l.Cleaning = firstString(ci, "description")
	}
	if cp, ok := m["cancellation_policy"].(map[string]any); ok {
		// Prefer short_description; fall back to joined description array / string.
		l.Cancellation = firstString(cp, "short_description", "name")
		if l.Cancellation == "" {
			l.Cancellation = joinStringish(cp["description"])
		} else if name := firstString(cp, "name"); name != "" && !strings.Contains(l.Cancellation, name) {
			l.Cancellation = name + " — " + l.Cancellation
		}
	}
	// "Included" is not always a dedicated field; use structured amenity names present (not missing).
	l.Included = extractIncluded(m)
	if loc, ok := m["location"].(map[string]any); ok {
		l.Lat = firstFloat(loc, "lat", "latitude")
		l.Lon = firstFloat(loc, "lon", "lng", "longitude")
	}
	if l.Lat == 0 && l.Lon == 0 {
		l.Lat = firstFloat(m, "latitude", "lat")
		l.Lon = firstFloat(m, "longitude", "lon", "lng")
	}
	// Hydrated marker: full detail payloads carry rules/parking/prices arrays.
	if _, hasRules := m["rules"]; hasRules {
		l.Hydrated = true
	}
	if _, hasParking := m["parking_info"]; hasParking {
		l.Hydrated = true
	}
	l.FormatFit = InferFormatFit(l)
	// A row with neither id nor title is not a listing.
	if l.ID == "" && l.Title == "" && l.PriceHourly == 0 && l.Guests == 0 {
		return Listing{}, false
	}
	return l, true
}

func extractPrice(m map[string]any) float64 {
	// Nested detailed_pricing.space_rental.booking_rate
	if dp, ok := m["detailed_pricing"].(map[string]any); ok {
		if sr, ok := dp["space_rental"].(map[string]any); ok {
			if v := firstFloat(sr, "booking_rate", "hourly", "rate"); v > 0 {
				return v
			}
		}
		// Flat detailed_pricing.booking_rate (observed in live search hits)
		if v := firstFloat(dp, "booking_rate", "price_hourly", "hourly"); v > 0 {
			return v
		}
	}
	// Full listing detail: prices: [{type: HOURLY_RATE, value: 85, currency: EUR}]
	if arr, ok := m["prices"].([]any); ok {
		for _, item := range arr {
			pm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			typ := strings.ToUpper(firstString(pm, "type"))
			if typ == "" || strings.Contains(typ, "HOURLY") {
				if v := firstFloat(pm, "value", "amount", "hourly"); v > 0 {
					return v
				}
			}
		}
	}
	// Literal dotted keys used by Peerspace search index
	if v := firstFloat(m, "price.hourly", "price_hourly", "priceHourly", "subtotal", "hourly_price"); v > 0 {
		return v
	}
	if p, ok := m["price"].(map[string]any); ok {
		if v := firstFloat(p, "hourly", "booking_rate", "amount"); v > 0 {
			return v
		}
	}
	if v, ok := m["price"].(float64); ok && v > 0 {
		return v
	}
	return 0
}

func extractCurrency(m map[string]any) string {
	if arr, ok := m["prices"].([]any); ok {
		for _, item := range arr {
			if pm, ok := item.(map[string]any); ok {
				if c := firstString(pm, "currency"); c != "" {
					return c
				}
			}
		}
	}
	return firstString(m, "price.currency", "currency")
}

func extractIncluded(m map[string]any) string {
	// Prefer explicit included fields when present.
	if s := firstString(m, "included", "included_in_booking", "whats_included"); s != "" {
		return s
	}
	// structured_amenities: present items (missing != true)
	names := make([]string, 0)
	if arr, ok := m["structured_amenities"].([]any); ok {
		for _, item := range arr {
			pm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if missing, ok := pm["missing"].(bool); ok && missing {
				continue
			}
			if n := firstString(pm, "localizedName", "name"); n != "" {
				names = append(names, n)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	max := 12
	if len(names) < max {
		max = len(names)
	}
	return strings.Join(names[:max], ", ")
}

func joinStringish(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func extractAmenities(m map[string]any) []string {
	out := make([]string, 0)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		out = append(out, s)
	}
	for _, key := range []string{"canonical_amenities", "amenities", "badges"} {
		raw, ok := m[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case []any:
			for _, item := range v {
				switch t := item.(type) {
				case string:
					add(t)
				case map[string]any:
					if dn := firstString(t, "display_name", "name", "id", "label"); dn != "" {
						add(dn)
					}
				}
			}
		case []string:
			for _, s := range v {
				add(s)
			}
		}
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				// numeric ids sometimes arrive as numbers
				if t == math.Trunc(t) {
					return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(
						jsonNumber(t), ".0"), ".00"))
				}
			}
		}
	}
	return ""
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func firstInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case int64:
				return int(t)
			case json.Number:
				i, _ := t.Int64()
				return int(i)
			case string:
				var n float64
				if err := json.Unmarshal([]byte(t), &n); err == nil {
					return int(n)
				}
			}
		}
	}
	return 0
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return t
			case int:
				return float64(t)
			case int64:
				return float64(t)
			case json.Number:
				f, _ := t.Float64()
				return f
			case string:
				var n float64
				if err := json.Unmarshal([]byte(t), &n); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				switch strings.ToLower(strings.TrimSpace(t)) {
				case "true", "1", "yes":
					return true
				}
			case float64:
				return t != 0
			}
		}
	}
	return false
}

// MatchCity reports whether listing city matches a filter (case-insensitive substring).
func MatchCity(l Listing, city string) bool {
	city = strings.TrimSpace(city)
	if city == "" {
		return true
	}
	return strings.Contains(strings.ToLower(l.City), strings.ToLower(city)) ||
		strings.Contains(strings.ToLower(l.Neighborhood), strings.ToLower(city)) ||
		strings.Contains(strings.ToLower(l.Title), strings.ToLower(city))
}

// knownUseIDs are Peerspace search use_id values. They label the market
// snapshot, not listing body text — so they do not exclude rows offline.
var knownUseIDs = map[string]struct{}{
	"meetup": {}, "event": {}, "party": {}, "film": {}, "film_shoot": {},
	"photoshoot": {}, "photo": {}, "workshop": {}, "wedding": {},
	"meeting": {}, "production": {}, "dining": {}, "popup": {},
}

// MatchActivity filters loosely on description/space type/title.
// Known Peerspace use_id values (meetup, event, …) pass through: they are
// search-channel tags, not listing fields, so requiring them in title/description
// empties every offline market cut.
func MatchActivity(l Listing, activity string) bool {
	activity = strings.TrimSpace(activity)
	if activity == "" {
		return true
	}
	a := strings.ToLower(activity)
	a = strings.ReplaceAll(a, "-", "_")
	a = strings.ReplaceAll(a, " ", "_")
	if _, ok := knownUseIDs[a]; ok {
		return true
	}
	blob := strings.ToLower(strings.Join([]string{l.Title, l.Description, l.SpaceType}, " "))
	return strings.Contains(blob, a) || strings.Contains(blob, strings.ReplaceAll(a, "_", " "))
}

// FilterListings applies city + activity filters.
func FilterListings(in []Listing, city, activity string) []Listing {
	out := make([]Listing, 0, len(in))
	for _, l := range in {
		if MatchCity(l, city) && MatchActivity(l, activity) {
			out = append(out, l)
		}
	}
	return out
}

// DedupeByID keeps the first occurrence of each listing id.
func DedupeByID(in []Listing) []Listing {
	seen := make(map[string]struct{}, len(in))
	out := make([]Listing, 0, len(in))
	for _, l := range in {
		key := l.ID
		if key == "" {
			key = l.Title + "|" + l.City
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, l)
	}
	return out
}
