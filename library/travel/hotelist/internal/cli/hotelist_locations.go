// Hand-authored location/chain resolution for the Hotelist CLI. Resolves a
// user-supplied "city, country, or region" token into the right /api filter,
// backed by a locally-synced city->geohash table scraped from hotelist.com's
// own <option> lists. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/store"
)

// ---- chain code map (parent chains; from hotelist.com chain dropdown) ----

var chainCodeToName = map[string]string{
	"BI": "1 Hotels", "EM": "Marriott", "EH": "Hilton", "HY": "Hyatt",
	"FS": "Four Seasons", "RZ": "Ritz-Carlton", "RT": "Accor", "6C": "IHG",
	"WR": "Wyndham", "CW": "Radisson", "BW": "Best Western", "WW": "WorldHotels",
	"NN": "Louvre", "SM": "Melia", "NK": "Okura", "DC": "Dorchester",
	"BY": "Banyan", "KI": "Kempinski", "PH": "Preferred", "EC": "Choice",
	"NH": "Minor", "RW": "Rosewood", "AM": "Aman", "CV": "Como", "PN": "Peninsula",
	"NB": "Nobu", "VY": "Maybourne", "AU": "Auberge", "CO": "Capella",
	"VG": "Viceroy", "AP": "Standard", "CL": "Cheval Blanc", "BG": "Bulgari",
	"MO": "Mandarin Oriental", "CU": "citizenM", "SS": "Sonder", "UV": "Motel One",
	"EN": "Ennismore", "BS": "Sercotel",
}

// chainNameToCode is the lowercased reverse map, built once.
var chainNameToCode = func() map[string]string {
	m := make(map[string]string, len(chainCodeToName))
	for code, name := range chainCodeToName {
		m[strings.ToLower(name)] = code
	}
	// a few friendly aliases
	m["ritz carlton"] = "RZ"
	m["mandarin"] = "MO"
	m["intercontinental"] = "6C"
	return m
}()

func chainDisplay(code string) string {
	if name, ok := chainCodeToName[code]; ok {
		return name + " (" + code + ")"
	}
	return code
}

// normalizeChain accepts a chain code (EM) or a chain name (Marriott, case-
// insensitive) and returns the upstream parent_chain_code. ok=false when the
// chain is not recognized.
func normalizeChain(input string) (code, display string, ok bool) {
	in := strings.TrimSpace(input)
	if in == "" {
		return "", "", false
	}
	upper := strings.ToUpper(in)
	if name, found := chainCodeToName[upper]; found {
		return upper, name + " (" + upper + ")", true
	}
	if c, found := chainNameToCode[strings.ToLower(in)]; found {
		return c, chainCodeToName[c] + " (" + c + ")", true
	}
	return "", "", false
}

// ---- region bounding boxes (continents; the site flies the map to these) ----

var regionBbox = map[string][4]float64{
	"europe":        {34, 72, -25, 45},
	"asia":          {5, 55, 60, 150},
	"africa":        {-35, 38, -18, 52},
	"north america": {15, 72, -168, -52},
	"latin america": {-56, 33, -118, -34},
	"middle east":   {12, 42, 25, 63},
	"oceania":       {-50, 0, 110, 180},
}

// ---- city table ----

type cityRecord struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Geohash string `json:"geohash"`
	Country string `json:"country"`
	Region  string `json:"region"`
}

// Hotelist renders each city <option> across several whitespace-separated
// lines, so every gap must be \s+ (which includes newlines) and the pattern
// runs in dot-all mode.
var optionRe = regexp.MustCompile(`(?s)<option\s+value="([a-z0-9]{2,12})"\s+data-country="([^"]*)"\s+data-region="([^"]*)"\s*>\s*([^<]+?)\s*</option>`)

// countryRe matches the homepage country-selector options, which carry a
// data-bbox + data-slug instead of data-country/data-region.
var countryRe = regexp.MustCompile(`(?s)<option\s+value="([^"]+)"\s+data-bbox="[^"]*"\s+data-slug="([^"]*)"\s*>\s*([^<]+?)\s*</option>`)

type countryRecord struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func makeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// syncCities scrapes and upserts the city AND country tables into the local
// store from one homepage fetch. Returns the number of cities stored.
func syncCities(ctx context.Context, c *client.Client, db *store.Store) (int, error) {
	data, err := c.GetNoCache(ctx, "/", nil)
	if err != nil {
		return 0, err
	}
	html := string(data)

	cities := parseCities(html)
	if len(cities) == 0 {
		return 0, fmt.Errorf("no cities found on hotelist.com homepage (site layout may have changed)")
	}
	for _, city := range cities {
		raw, err := json.Marshal(city)
		if err != nil {
			continue
		}
		if err := db.Upsert("city", city.Slug, raw); err != nil {
			return 0, fmt.Errorf("storing city %q: %w", city.Name, err)
		}
	}

	// Countries are best-effort: a parse miss here must not fail the city sync.
	for _, ctry := range parseCountries(html) {
		raw, err := json.Marshal(ctry)
		if err != nil {
			continue
		}
		_ = db.Upsert("country", strings.ToLower(ctry.Name), raw)
	}
	return len(cities), nil
}

func parseCities(html string) []cityRecord {
	matches := optionRe.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	out := make([]cityRecord, 0, len(matches))
	for _, m := range matches {
		geohash, country, region, name := m[1], m[2], m[3], strings.TrimSpace(m[4])
		if name == "" || country == "" {
			continue
		}
		key := strings.ToLower(name) + "|" + geohash
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cityRecord{Name: name, Slug: makeSlug(name), Geohash: geohash, Country: country, Region: region})
	}
	return out
}

func parseCountries(html string) []countryRecord {
	matches := countryRe.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	out := make([]countryRecord, 0, len(matches))
	for _, m := range matches {
		value, slug, text := m[1], m[2], strings.TrimSpace(m[3])
		name := value
		if name == "" {
			name = text
		}
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, countryRecord{Name: name, Slug: slug})
	}
	return out
}

// ensureCities syncs the city table if it is empty. Idempotent and cheap once
// populated.
func ensureCities(ctx context.Context, c *client.Client, db *store.Store) error {
	n, err := db.Count("city")
	if err == nil && n > 0 {
		return nil
	}
	_, err = syncCities(ctx, c, db)
	return err
}

// loadCity looks up a city by name or slug (case-insensitive) from the store.
func loadCity(db *store.Store, token string) (*cityRecord, bool) {
	slug := makeSlug(token)
	if raw, err := db.Get("city", slug); err == nil && len(raw) > 0 {
		var cr cityRecord
		if json.Unmarshal(raw, &cr) == nil {
			return &cr, true
		}
	}
	return nil, false
}

// loadCountry resolves a token to a canonical country value from the stored
// country table (by name or slug, case-insensitive). Falls back to the
// city-table country set so resolution still works if the country sync missed.
func loadCountry(db *store.Store, token string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(token))
	if raw, err := db.Get("country", lower); err == nil && len(raw) > 0 {
		var cr countryRecord
		if json.Unmarshal(raw, &cr) == nil {
			return cr.Name, true
		}
	}
	// slug match across stored countries
	rows, err := db.List("country", 100000)
	if err == nil {
		slug := makeSlug(token)
		for _, raw := range rows {
			var cr countryRecord
			if json.Unmarshal(raw, &cr) == nil && (cr.Slug == slug || strings.ToLower(cr.Name) == lower) {
				return cr.Name, true
			}
		}
	}
	// fallback: countries derived from the city table
	cityRows, err := db.List("city", 100000)
	if err == nil {
		for _, raw := range cityRows {
			var c cityRecord
			if json.Unmarshal(raw, &c) == nil && strings.ToLower(c.Country) == lower {
				return c.Country, true
			}
		}
	}
	return "", false
}

// resolvedLocation is the result of turning a user token into /api filters.
type resolvedLocation struct {
	Filters []apiFilter
	Label   string
	Kind    string // "city" | "country" | "region"
	Note    string
}

// resolveLocation turns a "city, country, or region" token into the right /api
// filter(s). It ensures the city table is populated first.
func resolveLocation(ctx context.Context, c *client.Client, db *store.Store, token string) (*resolvedLocation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("location is required (a city, country, or region)")
	}
	if err := ensureCities(ctx, c, db); err != nil {
		// Non-fatal: fall through to country/region matching even if the
		// homepage scrape failed.
		_ = err
	}
	lower := strings.ToLower(token)

	// 1) region (continent) match
	if bbox, ok := regionBbox[lower]; ok {
		return &resolvedLocation{
			Filters: []apiFilter{filterBbox(bbox)},
			Label:   strings.Title(lower),
			Kind:    "region",
		}, nil
	}

	// 2) exact city match
	if city, ok := loadCity(db, token); ok {
		return &resolvedLocation{
			Filters: []apiFilter{filterGeohash(city.Geohash)},
			Label:   city.Name + ", " + city.Country,
			Kind:    "city",
		}, nil
	}

	// 3) country match (stored country table, with city-table fallback)
	if canon, ok := loadCountry(db, token); ok {
		return &resolvedLocation{
			Filters: []apiFilter{filterCountry(canon)},
			Label:   canon,
			Kind:    "country",
		}, nil
	}

	// 4) unknown: error with suggestions rather than silently returning empty.
	msg := fmt.Sprintf("unknown location %q — not a known Hotelist city, country, or region", token)
	if sug := suggestCities(db, token, 5); len(sug) > 0 {
		msg += "\nDid you mean: " + strings.Join(sug, ", ") + "?"
	} else {
		msg += "\nRun 'hotelist-pp-cli sync cities' to refresh the local table, or check the spelling."
	}
	return nil, usageErr(fmt.Errorf("%s", msg))
}

// suggestCities returns up to n city names containing the substring, for
// helpful error messages.
func suggestCities(db *store.Store, token string, n int) []string {
	rows, err := db.List("city", 100000)
	if err != nil {
		return nil
	}
	lower := strings.ToLower(token)
	var out []string
	for _, raw := range rows {
		var cr cityRecord
		if json.Unmarshal(raw, &cr) == nil && strings.Contains(strings.ToLower(cr.Name), lower) {
			out = append(out, cr.Name)
		}
	}
	sort.Strings(out)
	if len(out) > n {
		out = out[:n]
	}
	return out
}
