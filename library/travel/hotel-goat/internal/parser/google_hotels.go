// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

// Package parser extracts structured hotel records from Google Hotels' SSR
// HTML. Google embeds two AF_initDataCallback(key:'ds:N', ...) blobs per
// page; ds:0 on the search page (or ds:1 on the detail page) carries the
// hotel records as deeply nested arrays. This package walks the nested
// shape defensively (every index access is bounds-checked) so a Google
// reshuffle degrades gracefully rather than panics.
package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ParserVersion bumps when the field-extraction map changes shape. Allows
// downstream diagnostics to detect when the parser needs an update against
// a fresh Google SSR sample.
const ParserVersion = "2026-05-23.1"

// Hotel is the public, agent-friendly record this parser emits. Mirrors
// the field names declared in hotel-goat-spec.yaml so the existing
// generated client envelope can serialize it without renames.
type Hotel struct {
	PropertyToken string      `json:"property_token,omitempty"`
	Name          string      `json:"name,omitempty"`
	Brand         string      `json:"brand,omitempty"`
	Address       string      `json:"address,omitempty"`
	Latitude      float64     `json:"latitude,omitempty"`
	Longitude     float64     `json:"longitude,omitempty"`
	HotelClass    int         `json:"hotel_class,omitempty"`
	Rating        float64     `json:"rating,omitempty"`
	Reviews       int         `json:"reviews,omitempty"`
	PricePerNight float64     `json:"price_per_night,omitempty"`
	Currency      string      `json:"currency,omitempty"`
	Amenities     []string    `json:"amenities,omitempty"`
	Prices        []OTAPrice  `json:"prices,omitempty"`
	BookingURLs   BookingURLs `json:"booking_urls,omitempty"`
	Images        []string    `json:"images,omitempty"`
	Description   string      `json:"description,omitempty"`
	Thumbnail     string      `json:"thumbnail,omitempty"`

	// NearbyDistanceMiles is populated only by the `near` command when
	// --radius is set. Distance from the user's center pivot to this
	// hotel's lat/lng, in statute miles.
	NearbyDistanceMiles float64 `json:"nearby_distance_miles,omitempty"`
}

type OTAPrice struct {
	Source string  `json:"source,omitempty"`
	Price  float64 `json:"price,omitempty"`
	Link   string  `json:"link,omitempty"`
	Logo   string  `json:"logo,omitempty"`
}

type BookingURLs struct {
	Primary   string `json:"primary,omitempty"`
	HotelURL  string `json:"hotel_url,omitempty"`
	GoogleURL string `json:"google_url,omitempty"`
}

// initDataRE matches the AF_initDataCallback bootstrap blob prefix. The
// JSON `data:` value is the bracketed array immediately after — we scan
// brackets after the prefix rather than regex-matching to handle deeply
// nested arrays and embedded strings that would defeat a single regex.
var initDataRE = regexp.MustCompile(`AF_initDataCallback\(\{key: '(ds:\d+)', hash: '[^']*', data:`)

// ExtractInitDataBlobs walks the raw HTML and returns each AF_initDataCallback
// payload as ds-key -> JSON-array bytes. Caller decides which blob(s) to
// JSON-unmarshal. Empty map (not error) when no callbacks are found —
// that's the signal we hit a captcha/login wall rather than a real page.
func ExtractInitDataBlobs(html []byte) map[string][]byte {
	out := map[string][]byte{}
	matches := initDataRE.FindAllSubmatchIndex(html, -1)
	for _, m := range matches {
		key := string(html[m[2]:m[3]])
		pos := m[1]
		// Skip to first '['
		for pos < len(html) && html[pos] != '[' {
			pos++
		}
		if pos >= len(html) {
			continue
		}
		start := pos
		depth := 0
		inStr := false
		i := pos
		for i < len(html) {
			c := html[i]
			if inStr {
				if c == '\\' {
					i += 2
					continue
				}
				if c == '"' {
					inStr = false
				}
			} else {
				switch c {
				case '"':
					inStr = true
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						out[key] = append([]byte(nil), html[start:i+1]...)
						goto done
					}
				}
			}
			i++
		}
	done:
	}
	return out
}

// ParseSearchPage extracts the property records visible on the
// /travel/search SSR. Defensive walk: every index lookup is bounds-checked
// so a Google reshuffle degrades to "fewer fields populated" rather than
// a panic.
func ParseSearchPage(html []byte) ([]Hotel, error) {
	blobs := ExtractInitDataBlobs(html)
	if len(blobs) == 0 {
		return nil, fmt.Errorf("no AF_initDataCallback blobs found (captcha/login wall?)")
	}
	var hotels []Hotel
	seen := map[string]bool{}
	// Walk every blob looking for the type-34 hotel record pattern.
	// Google reshuffles which ds:N holds the data, so we don't hard-code.
	for _, blob := range blobs {
		var node any
		if err := json.Unmarshal(blob, &node); err != nil {
			continue
		}
		walkForHotels(node, func(h Hotel) {
			if h.PropertyToken != "" && seen[h.PropertyToken] {
				return
			}
			if h.Name == "" {
				return
			}
			if h.PropertyToken != "" {
				seen[h.PropertyToken] = true
			}
			hotels = append(hotels, h)
		})
	}
	return hotels, nil
}

// ParseDetailPage extracts the focal property's full detail from the
// /travel/hotels/entity/{token} SSR.
func ParseDetailPage(html []byte) (*Hotel, error) {
	blobs := ExtractInitDataBlobs(html)
	if len(blobs) == 0 {
		return nil, fmt.Errorf("no AF_initDataCallback blobs found (captcha/login wall?)")
	}
	for _, blob := range blobs {
		var node any
		if err := json.Unmarshal(blob, &node); err != nil {
			continue
		}
		var focal *Hotel
		walkForHotels(node, func(h Hotel) {
			if focal == nil {
				h2 := h
				focal = &h2
			}
		})
		if focal != nil {
			// Try to enrich with OTA breakdown specific to detail-page shape.
			otaList := findDetailOTAs(node)
			if len(otaList) > 0 {
				focal.Prices = otaList
			}
			return focal, nil
		}
	}
	return nil, fmt.Errorf("no property records found in detail page")
}

// walkForHotels traverses an arbitrary JSON value tree and calls emit
// for each value that matches the [34, {"397419284": [...]}] hotel-card
// pattern Google uses inside ds:0/ds:1.
func walkForHotels(node any, emit func(Hotel)) {
	switch v := node.(type) {
	case []any:
		// Match: [34, {"397419284": [...]}]
		if len(v) == 2 && toInt(v[0]) == 34 {
			if m, ok := v[1].(map[string]any); ok {
				for _, payload := range m {
					if h, ok := tryParseHotel(payload); ok {
						emit(h)
					}
				}
			}
		}
		for _, c := range v {
			walkForHotels(c, emit)
		}
	case map[string]any:
		for _, c := range v {
			walkForHotels(c, emit)
		}
	}
}

// tryParseHotel turns the payload at hotel-card-shape into a Hotel.
// payload is `[[null, name, [...meta...], [class_label, class_int], ...]]`
// (a single-element outer wrapper around the real record list).
func tryParseHotel(payload any) (Hotel, bool) {
	wrapper, ok := payload.([]any)
	if !ok || len(wrapper) == 0 {
		return Hotel{}, false
	}
	rec, ok := wrapper[0].([]any)
	if !ok || len(rec) < 2 {
		return Hotel{}, false
	}
	name, _ := rec[1].(string)
	if strings.TrimSpace(name) == "" {
		return Hotel{}, false
	}
	h := Hotel{Name: name}

	// [2] = meta block with lat/lng and other goodies
	if len(rec) > 2 {
		if meta, ok := rec[2].([]any); ok {
			h.Latitude, h.Longitude = extractLatLng(meta)
			// meta[29][2] often carries the direct hotel URL
			if u := digString(meta, 29, 2); u != "" {
				h.BookingURLs.HotelURL = u
			}
		}
	}
	// [3] = ["4-star hotel", 4]
	if len(rec) > 3 {
		if cls, ok := rec[3].([]any); ok && len(cls) >= 2 {
			h.HotelClass = toInt(cls[1])
		}
	}
	// [5] = image bundle
	if len(rec) > 5 {
		h.Images = extractImages(rec[5])
	}
	// [6] = pricing block.
	//   [6][1][3] = currency code (e.g. "USD")
	//   [6][2][1] = ["$184", "$217", 184.02, null, 184]  -- display nightly rate
	if len(rec) > 6 {
		if priceBlock, ok := rec[6].([]any); ok {
			// Currency at [1][3]. Bounds-check priceBlock — Google's
			// price block has varied shape (sometimes a single nested
			// array, sometimes empty when no OTA returned a price).
			// Without the len() guard this panics on the short shape.
			if len(priceBlock) > 1 {
				if meta, ok := priceBlock[1].([]any); ok && len(meta) > 3 {
					if cur, ok := meta[3].(string); ok {
						h.Currency = cur
					}
				}
			}
			// Price at [2][1] when present.
			if len(priceBlock) > 2 {
				if detail, ok := priceBlock[2].([]any); ok && len(detail) > 1 {
					h.PricePerNight = parsePriceArray(detail[1])
				}
			}
		}
	}
	// [7][0] = [rating, review_count]
	if len(rec) > 7 {
		if r, ok := rec[7].([]any); ok && len(r) > 0 {
			if rr, ok := r[0].([]any); ok && len(rr) >= 2 {
				h.Rating = toFloat(rr[0])
				h.Reviews = toInt(rr[1])
			}
		}
	}
	// [11][0] = description
	if len(rec) > 11 {
		if dl, ok := rec[11].([]any); ok && len(dl) > 0 {
			if s, ok := dl[0].(string); ok {
				h.Description = s
			}
		}
	}
	// [12][0] = thumbnail
	if len(rec) > 12 {
		if dl, ok := rec[12].([]any); ok && len(dl) > 0 {
			if s, ok := dl[0].(string); ok {
				h.Thumbnail = s
			}
		}
	}
	// [20] = property_token (the ChcI... base64-like string)
	if len(rec) > 20 {
		if s, ok := rec[20].(string); ok && strings.HasPrefix(s, "Ch") {
			h.PropertyToken = s
		}
	}

	// Brand inferred from name against well-known chains. (Detail-page
	// extraction can later replace this with a stored brand id.)
	h.Brand = inferBrand(name)

	// Google entity URL is always derivable when we have a token.
	if h.PropertyToken != "" {
		h.BookingURLs.GoogleURL = "https://www.google.com/travel/hotels/entity/" + h.PropertyToken
	}
	// Primary URL preference: cheapest OTA > hotel direct > Google detail.
	h.BookingURLs.Primary = pickPrimary(h.BookingURLs.HotelURL, h.BookingURLs.GoogleURL)

	return h, true
}

// extractLatLng pulls lat/lng from the meta block at meta[0]=[lat, lng].
func extractLatLng(meta []any) (float64, float64) {
	if len(meta) == 0 {
		return 0, 0
	}
	coord, ok := meta[0].([]any)
	if !ok || len(coord) < 2 {
		return 0, 0
	}
	lat, _ := coord[0].(float64)
	lng, _ := coord[1].(float64)
	return lat, lng
}

// extractImages walks the image bundle returning URL strings.
func extractImages(node any) []string {
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case []any:
			for _, c := range v {
				walk(c)
			}
		case map[string]any:
			for _, c := range v {
				walk(c)
			}
		case string:
			if strings.HasPrefix(v, "https://") && (strings.Contains(v, "googleusercontent") || strings.Contains(v, "gstatic")) {
				out = append(out, v)
			}
		}
	}
	walk(node)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// digString safely indexes into a nested any tree returning the first
// string at the given path, or "" if any hop fails.
func digString(root any, path ...int) string {
	cur := root
	for _, idx := range path {
		arr, ok := cur.([]any)
		if !ok || idx < 0 || idx >= len(arr) {
			return ""
		}
		cur = arr[idx]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

// parsePriceArray reads `["$184", "$217", 184.02, ...]` returning the
// nightly USD-equivalent rate. Falls back to parsing the leading
// display-string ($184) when the numeric float index is missing.
func parsePriceArray(node any) float64 {
	arr, ok := node.([]any)
	if !ok {
		return 0
	}
	// Prefer the float at index 2 (rate as number, before strikethrough).
	if len(arr) > 2 {
		if f := toFloat(arr[2]); f != 0 {
			return f
		}
	}
	if len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return parseDollarString(s)
		}
	}
	return 0
}

// toInt accepts any numeric variant json.Unmarshal may produce (float64) and
// also handles int from hand-built fixtures. Returns 0 on miss.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func parseDollarString(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// findDetailOTAs locates the per-source OTA breakdown on a detail page.
// Google places it at d[0][6][2][21]: a list of `[[source, _, link, [logo], ...] ..., [..., [..., [price_arr,...], ...]]]`.
func findDetailOTAs(root any) []OTAPrice {
	// Defensive descent
	cur := root
	for _, idx := range []int{0, 6, 2, 21} {
		arr, ok := cur.([]any)
		if !ok || idx >= len(arr) {
			return nil
		}
		cur = arr[idx]
	}
	list, ok := cur.([]any)
	if !ok {
		return nil
	}
	var out []OTAPrice
	for _, entry := range list {
		e, ok := entry.([]any)
		if !ok || len(e) == 0 {
			continue
		}
		head, ok := e[0].([]any)
		if !ok || len(head) < 3 {
			continue
		}
		name, _ := head[0].(string)
		link, _ := head[2].(string)
		var logo string
		if len(head) > 3 {
			if logos, ok := head[3].([]any); ok && len(logos) > 0 {
				logo, _ = logos[0].(string)
			}
		}
		var price float64
		if len(e) > 14 {
			if priceCell, ok := e[14].([]any); ok && len(priceCell) > 4 {
				price = parsePriceArray(priceCell[4])
			}
		}
		if name == "" {
			continue
		}
		out = append(out, OTAPrice{
			Source: name,
			Price:  price,
			Link:   absolutizeGoogle(link),
			Logo:   absolutizeProtocol(logo),
		})
	}
	return out
}

func absolutizeGoogle(link string) string {
	if link == "" {
		return ""
	}
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	if strings.HasPrefix(link, "//") {
		return "https:" + link
	}
	if strings.HasPrefix(link, "/") {
		return "https://www.google.com" + link
	}
	return link
}

func absolutizeProtocol(s string) string {
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	return s
}

func pickPrimary(hotelURL, googleURL string) string {
	if hotelURL != "" {
		return hotelURL
	}
	return googleURL
}

// brandPrefixes maps loyalty programs to their visible sub-brand names.
// This is a hard-coded fallback; the canonical list lives in the DB's
// brand_aliases table populated by store/hotel_goat_migrations.go.
var brandPrefixes = map[string][]string{
	"Hyatt":    {"Park Hyatt", "Andaz", "Thompson", "Hyatt Place", "Hyatt House", "Hyatt Centric", "Grand Hyatt", "Hyatt Regency", "Alila", "Miraval", "Hyatt"},
	"Marriott": {"Marriott", "Courtyard", "Residence Inn", "Westin", "Sheraton", "Renaissance", "Le Méridien", "JW Marriott", "Ritz-Carlton", "St. Regis", "Tribute Portfolio", "Autograph Collection", "Aloft", "Element", "Moxy", "AC Hotels", "Delta Hotels", "Four Points", "Fairfield", "SpringHill Suites", "TownePlace Suites"},
	"Hilton":   {"Hilton", "DoubleTree", "Hampton", "Embassy Suites", "Hilton Garden Inn", "Homewood Suites", "Home2", "Tru", "Curio", "Tapestry", "Canopy", "Conrad", "Waldorf Astoria", "LXR", "Signia", "Motto"},
	"IHG":      {"Holiday Inn Express", "Holiday Inn", "Crowne Plaza", "InterContinental", "Kimpton", "Hotel Indigo", "Voco", "EVEN", "Avid", "Staybridge", "Candlewood", "Six Senses", "Regent"},
	"Accor":    {"Sofitel", "Pullman", "Novotel", "Mercure", "ibis", "Raffles", "Fairmont", "MGallery", "SO/", "25hours", "Mama Shelter"},
}

func inferBrand(name string) string {
	lower := strings.ToLower(name)
	// Go map iteration is randomized, so iterating brandPrefixes directly
	// produces non-deterministic matches: "JW Marriott" could match
	// "JW Marriott" or just "Marriott" depending on the run. Flatten to
	// a slice and sort by descending length so longest-prefix wins,
	// which is both deterministic AND semantically correct (the more
	// specific sub-brand should beat the parent chain name).
	type candidate struct{ sub string }
	all := make([]candidate, 0, 64)
	for _, subs := range brandPrefixes {
		for _, sub := range subs {
			all = append(all, candidate{sub: sub})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		return len(all[i].sub) > len(all[j].sub)
	})
	for _, c := range all {
		if strings.Contains(lower, strings.ToLower(c.sub)) {
			return c.sub
		}
	}
	return ""
}

// ProgramForBrand returns the loyalty program (hyatt, marriott, hilton, ihg, accor)
// for a sub-brand name. Empty when unmatched.
func ProgramForBrand(sub string) string {
	lower := strings.ToLower(sub)
	for program, subs := range brandPrefixes {
		for _, s := range subs {
			if strings.EqualFold(s, lower) || strings.Contains(lower, strings.ToLower(s)) {
				return strings.ToLower(program)
			}
		}
	}
	return ""
}

// BrandsForProgram returns all sub-brands for the named loyalty program.
func BrandsForProgram(program string) []string {
	p := ""
	for k := range brandPrefixes {
		if strings.EqualFold(k, program) {
			p = k
			break
		}
	}
	if p == "" {
		return nil
	}
	return append([]string(nil), brandPrefixes[p]...)
}
