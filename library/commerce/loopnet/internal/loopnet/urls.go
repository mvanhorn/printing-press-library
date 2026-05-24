package loopnet

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// BaseURL is the LoopNet origin every request and stored URL is built from.
const BaseURL = "https://www.loopnet.com"

var reSlugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// typeAliases maps user-friendly property-type words to LoopNet's URL slug.
// Unknown input is slugified and passed through, so an exact LoopNet slug
// the table doesn't list still works.
var typeAliases = map[string]string{
	"office": "office", "offices": "office", "office-buildings": "office",
	"office-building": "office", "office-space": "office",
	"industrial": "industrial", "industrial-properties": "industrial",
	"warehouse": "industrial", "warehouses": "industrial", "flex": "industrial",
	"retail": "retail", "retail-space": "retail", "restaurant": "retail",
	"restaurants": "retail", "shopping-center": "retail",
	"multifamily": "multifamily", "multi-family": "multifamily",
	"apartment": "multifamily", "apartments": "multifamily",
	"apartment-buildings": "multifamily",
	"land":                "land",
	"hospitality":         "hospitality", "hotel": "hospitality", "hotels": "hospitality",
	"health-care": "health-care", "healthcare": "health-care",
	"medical":   "health-care",
	"specialty": "specialty", "special-purpose": "specialty",
	"all": "commercial-real-estate", "any": "commercial-real-estate",
	"commercial":             "commercial-real-estate",
	"commercial-real-estate": "commercial-real-estate",
}

// PropertyTypes lists the canonical LoopNet property-type slugs the CLI
// accepts directly (aliases also resolve via NormalizeType).
var PropertyTypes = []string{
	"office", "industrial", "retail", "multifamily", "land",
	"hospitality", "health-care", "specialty", "commercial-real-estate",
}

// NormalizeType resolves a user-supplied property type to a LoopNet URL
// slug. Falls back to slugifying unknown input so an exact slug still works.
func NormalizeType(input string) string {
	key := slugify(input)
	if key == "" {
		return "commercial-real-estate"
	}
	if s, ok := typeAliases[key]; ok {
		return s
	}
	return key
}

// NormalizeListingType resolves a user-supplied listing type to either
// "for-sale" or "for-lease". Defaults to for-sale.
func NormalizeListingType(input string) string {
	switch slugify(input) {
	case "for-lease", "lease", "rent", "for-rent", "lease-rent":
		return "for-lease"
	case "businesses-for-sale", "business-for-sale", "business", "businesses":
		return "businesses-for-sale"
	default:
		return "for-sale"
	}
}

// SlugLocation normalizes a location ("Worcester, MA", "worcester ma",
// "01608") to a LoopNet location slug ("worcester-ma", "01608").
func SlugLocation(input string) string {
	return slugify(input)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reSlugStrip.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// SearchFilters are the optional price/size narrowing parameters LoopNet
// accepts as query string arguments on a search URL.
type SearchFilters struct {
	MinPrice  int
	MaxPrice  int
	PriceType string // "", "unit", "sf", "acre"
	MinSize   int
	MaxSize   int
}

// BuildSearchURL constructs an absolute LoopNet search URL for a property
// type, location, listing type, and (1-based) page number.
func BuildSearchURL(propertyType, location, listingType string, page int, f SearchFilters) string {
	pt := NormalizeType(propertyType)
	loc := SlugLocation(location)
	lt := NormalizeListingType(listingType)
	path := fmt.Sprintf("/search/%s/%s/%s/", pt, loc, lt)
	if page > 1 {
		path += fmt.Sprintf("%d/", page)
	}
	q := url.Values{}
	if f.MinPrice > 0 {
		q.Set("min-price", fmt.Sprintf("%d", f.MinPrice))
	}
	if f.MaxPrice > 0 {
		q.Set("max-price", fmt.Sprintf("%d", f.MaxPrice))
	}
	if f.PriceType != "" {
		q.Set("price-type", f.PriceType)
	}
	if f.MinSize > 0 {
		q.Set("min-size", fmt.Sprintf("%d", f.MinSize))
	}
	if f.MaxSize > 0 {
		q.Set("max-size", fmt.Sprintf("%d", f.MaxSize))
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return BaseURL + path
}

// BuildDetailURL constructs an absolute LoopNet detail URL from a listing id.
// LoopNet redirects /Listing/<id>/ to the canonical slug URL.
func BuildDetailURL(id string) string {
	id = strings.TrimSpace(id)
	return fmt.Sprintf("%s/Listing/%s/", BaseURL, id)
}

// SearchPath returns the BaseURL-relative search path (no query string) for
// use with the generated client, which prepends BaseURL itself.
func SearchPath(propertyType, location, listingType string, page int) string {
	path := fmt.Sprintf("/search/%s/%s/%s/",
		NormalizeType(propertyType), SlugLocation(location), NormalizeListingType(listingType))
	if page > 1 {
		path += fmt.Sprintf("%d/", page)
	}
	return path
}

// DetailPath returns the BaseURL-relative detail path for a listing id.
func DetailPath(id string) string {
	return "/Listing/" + strings.TrimSpace(id) + "/"
}

// PathOf strips the LoopNet origin from an absolute URL, returning a
// BaseURL-relative path. A path or non-LoopNet URL is returned unchanged.
func PathOf(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	for _, p := range []string{"https://www.loopnet.com", "http://www.loopnet.com", "https://loopnet.com"} {
		if strings.HasPrefix(rawURL, p) {
			return strings.TrimPrefix(rawURL, p)
		}
	}
	return rawURL
}

// FilterParams renders SearchFilters as the query-parameter map the
// generated client accepts alongside a path.
func (f SearchFilters) FilterParams() map[string]string {
	p := map[string]string{}
	if f.MinPrice > 0 {
		p["min-price"] = fmt.Sprintf("%d", f.MinPrice)
	}
	if f.MaxPrice > 0 {
		p["max-price"] = fmt.Sprintf("%d", f.MaxPrice)
	}
	if f.PriceType != "" {
		p["price-type"] = f.PriceType
	}
	if f.MinSize > 0 {
		p["min-size"] = fmt.Sprintf("%d", f.MinSize)
	}
	if f.MaxSize > 0 {
		p["max-size"] = fmt.Sprintf("%d", f.MaxSize)
	}
	return p
}

// MarketKey is the canonical store key for a synced submarket — a
// location + property type + listing type triple.
func MarketKey(location, propertyType, listingType string) string {
	return fmt.Sprintf("%s|%s|%s",
		SlugLocation(location), NormalizeType(propertyType), NormalizeListingType(listingType))
}
