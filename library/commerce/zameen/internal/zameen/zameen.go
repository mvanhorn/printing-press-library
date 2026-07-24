// Zameen domain helpers: location/category resolution, window.state parsing,
// and hit -> types.Listing mapping. Hand-authored (not generated): the listing
// data lives in a `window.state = {…}` JS blob on the server-rendered search
// page, which the generator's html_extract cannot parse.
package zameen

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

const (
	// BaseURL is the Zameen web origin. Search pages are server-rendered here.
	BaseURL = "https://www.zameen.com"
	// PageSize is the fixed number of listings Zameen returns per search page.
	PageSize = 25
	// AreaSqmPerMarla is Zameen's internal area unit conversion, derived
	// empirically and exactly from the data: every listing's `area` field
	// divided by its declared Marla is 20.903184. 1 Marla = 20.903184 m².
	AreaSqmPerMarla = 20.903184
	// UserAgent is sent on every request. Zameen sits behind Cloudflare but
	// serves plain HTTP 200 to a browser UA (no JS challenge).
	UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// knownCities maps a friendly city name to its authoritative Zameen location
// slug. The numeric id is what Zameen resolves on; the name part is cosmetic.
// Users can bypass this entirely with --location <slug> (any city or area slug
// copied from a Zameen URL, e.g. Lahore_DHA_Defence-9).
var knownCities = map[string]string{
	"islamabad":  "Islamabad-3",
	"lahore":     "Lahore-1",
	"karachi":    "Karachi-2",
	"rawalpindi": "Rawalpindi-41",
	"faisalabad": "Faisalabad-16",
	"multan":     "Multan-15",
}

// KnownCities returns the sorted friendly city names the CLI resolves directly.
func KnownCities() []string {
	return []string{"Islamabad", "Karachi", "Lahore", "Multan", "Rawalpindi", "Faisalabad"}
}

// ResolveLocation turns a --city name or a raw --location slug into the Zameen
// location slug used in the search URL. A raw slug (contains a digit id after a
// dash, e.g. "Islamabad-3" or "Lahore_DHA_Defence-9") is passed through as-is.
func ResolveLocation(city, location string) (string, error) {
	if strings.TrimSpace(location) != "" {
		return strings.TrimSpace(location), nil
	}
	key := strings.ToLower(strings.TrimSpace(city))
	if key == "" {
		return "", fmt.Errorf("provide --city (one of %s) or --location <slug>", strings.Join(KnownCities(), ", "))
	}
	if slug, ok := knownCities[key]; ok {
		return slug, nil
	}
	// Accept a slug-shaped value passed via --city too.
	if strings.ContainsRune(city, '-') {
		return city, nil
	}
	return "", fmt.Errorf("unknown city %q; use one of %s or pass --location <slug> from a Zameen URL", city, strings.Join(KnownCities(), ", "))
}

// ResolveCategory maps a purpose (buy/rent) and property type to the Zameen URL
// category segment. Rentals live under a single "Rentals" segment regardless of
// type; buy listings use the type segment. Type is still filtered client-side.
func ResolveCategory(purpose, propertyType string) string {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "rent", "rentals", "for-rent", "to-rent":
		return "Rentals"
	}
	switch strings.Title(strings.ToLower(strings.TrimSpace(propertyType))) {
	case "Plots", "Plot", "Land":
		return "Plots"
	case "Commercial", "Office", "Shop":
		return "Commercial"
	default:
		return "Homes"
	}
}

// NormalizePurpose maps user purpose input to Zameen's canonical value.
func NormalizePurpose(purpose string) string {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "rent", "rentals", "for-rent", "to-rent":
		return "rent"
	default:
		return "for-sale"
	}
}

// flexString unmarshals a JSON string OR number into a Go string, because
// Zameen encodes some ids as strings and some as numbers across responses.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		*f = ""
		return nil
	}
	if len(s) >= 2 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = flexString(str)
		return nil
	}
	*f = flexString(strings.Trim(s, `"`))
	return nil
}

type locCat struct {
	Name       string `json:"name"`
	ExternalID string `json:"externalID"`
	Level      int    `json:"level"`
}

// hit is one Zameen listing as it appears in window.state.algolia.content.hits.
type hit struct {
	ExternalID flexString `json:"externalID"`
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Purpose    string     `json:"purpose"`
	Price      int        `json:"price"`
	Area       float64    `json:"area"`
	Rooms      int        `json:"rooms"`
	Baths      int        `json:"baths"`
	IsVerified bool       `json:"isVerified"`
	Slug       string     `json:"slug"`
	Product    string     `json:"product"`
	CreatedAt  int        `json:"createdAt"`
	UpdatedAt  int        `json:"updatedAt"`
	Geography  struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"geography"`
	Category []locCat `json:"category"`
	Location []locCat `json:"location"`
	Agency   struct {
		Name string `json:"name"`
	} `json:"agency"`
}

// areaName returns the mid-level area (e.g. "DHA Defence", "B-17") used for
// grouping in comps/deals; falls back to the deepest location name.
func (h hit) areaName() string {
	var lvl3, deepest string
	maxLevel := -1
	for _, l := range h.Location {
		if l.Level == 3 {
			lvl3 = l.Name
		}
		if l.Level > maxLevel {
			maxLevel = l.Level
			deepest = l.Name
		}
	}
	if lvl3 != "" {
		return lvl3
	}
	return deepest
}

func (h hit) cityName() string {
	for _, l := range h.Location {
		if l.Level == 2 {
			return l.Name
		}
	}
	return ""
}

// matchesArea reports whether any name in the listing's location chain contains
// the given area query (case-insensitive, underscores treated as spaces).
func (h hit) matchesArea(area string) bool {
	q := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(area), "_", " "))
	if q == "" {
		return true
	}
	for _, l := range h.Location {
		if strings.Contains(strings.ToLower(l.Name), q) {
			return true
		}
	}
	return false
}

func (h hit) externalID() string {
	if s := string(h.ExternalID); s != "" {
		return s
	}
	if h.ID != 0 {
		return strconv.FormatInt(h.ID, 10)
	}
	return ""
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// toListing converts a raw hit to the exported Listing type stored and printed.
func (h hit) toListing() types.Listing {
	ptype := ""
	if len(h.Category) > 0 {
		ptype = h.Category[0].Name
	}
	marla := 0.0
	if h.Area > 0 {
		marla = round2(h.Area / AreaSqmPerMarla)
	}
	return types.Listing{
		ExternalId:   h.externalID(),
		Title:        cliutil.CleanText(h.Title),
		Purpose:      h.Purpose,
		PropertyType: ptype,
		Price:        h.Price,
		AreaMarla:    marla,
		Beds:         h.Rooms,
		Baths:        h.Baths,
		City:         h.cityName(),
		Location:     h.areaName(),
		Agency:       cliutil.CleanText(h.Agency.Name),
		IsVerified:   h.IsVerified,
		Lat:          h.Geography.Lat,
		Lng:          h.Geography.Lng,
		CreatedAt:    h.CreatedAt,
		UpdatedAt:    h.UpdatedAt,
		Url:          BaseURL + "/Property/" + h.Slug + ".html",
	}
}
