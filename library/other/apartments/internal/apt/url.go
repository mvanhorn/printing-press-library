// Package apt holds apartments.com-specific helpers: slug-URL composition,
// HTML parsing for placard summaries and listing detail pages, and the
// extension migrations + query helpers for the local SQLite store.
package apt

import (
	"fmt"
	"strings"
)

// SearchOptions describes one apartments.com path-slug search. The zero
// value (City/State only) is valid; every other field is optional.
type SearchOptions struct {
	City     string // lowercased, hyphens for spaces
	State    string // 2-letter lowercase
	Zip      string // overrides City+State if set
	Beds     int    // 0 = studio (when Studio true), else exact
	BedsMin  int    // mutually exclusive with Beds
	Studio   bool
	Baths    int
	BathsMin int
	PriceMin int
	PriceMax int
	Pets     string // "", "any", "cat", "dog", "both"
	Type     string // "", "apartment", "house", "condo", "townhome"
	Page     int
}

// BuildSearchURL renders a SearchOptions to an apartments.com path-slug
// relative URL like "/austin-tx/min-2-bedrooms-under-2500-pet-friendly/".
// The result always begins and ends with "/".
func BuildSearchURL(opts SearchOptions) string {
	var segments []string

	// Property-type prefix.
	switch strings.ToLower(opts.Type) {
	case "house":
		segments = append(segments, "houses")
	case "condo":
		segments = append(segments, "condos")
	case "townhome":
		segments = append(segments, "townhomes")
	}

	// Location.
	if opts.Zip != "" {
		segments = append(segments, opts.Zip)
	} else if opts.City != "" || opts.State != "" {
		loc := opts.City
		if opts.State != "" {
			if loc == "" {
				loc = opts.State
			} else {
				loc = loc + "-" + opts.State
			}
		}
		segments = append(segments, loc)
	}

	// Filters: beds → baths → price → pets.
	var filters []string
	if f := bedsFilter(opts); f != "" {
		filters = append(filters, f)
	}
	if f := bathsFilter(opts); f != "" {
		filters = append(filters, f)
	}
	if f := priceFilter(opts); f != "" {
		filters = append(filters, f)
	}
	if f := petsFilter(opts); f != "" {
		filters = append(filters, f)
	}
	if len(filters) > 0 {
		segments = append(segments, strings.Join(filters, "-"))
	}

	// Pagination (1-indexed; omit on page 1).
	if opts.Page > 1 {
		segments = append(segments, fmt.Sprintf("%d", opts.Page))
	}

	// Compose the path. Leading + trailing slash is the apartments.com
	// canonical form.
	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/") + "/"
}

func bedsFilter(o SearchOptions) string {
	if o.Studio && o.Beds == 0 && o.BedsMin == 0 {
		return "studio"
	}
	if o.Beds > 0 {
		return fmt.Sprintf("%d-bedrooms", o.Beds)
	}
	if o.BedsMin > 0 {
		return fmt.Sprintf("min-%d-bedrooms", o.BedsMin)
	}
	return ""
}

func bathsFilter(o SearchOptions) string {
	if o.Baths > 0 {
		return fmt.Sprintf("%d-bathrooms", o.Baths)
	}
	if o.BathsMin > 0 {
		return fmt.Sprintf("min-%d-bathrooms", o.BathsMin)
	}
	return ""
}

func priceFilter(o SearchOptions) string {
	if o.PriceMin > 0 && o.PriceMax > 0 {
		return fmt.Sprintf("%d-to-%d", o.PriceMin, o.PriceMax)
	}
	if o.PriceMax > 0 {
		return fmt.Sprintf("under-%d", o.PriceMax)
	}
	if o.PriceMin > 0 {
		return fmt.Sprintf("over-%d", o.PriceMin)
	}
	return ""
}

func petsFilter(o SearchOptions) string {
	switch strings.ToLower(o.Pets) {
	case "any":
		return "pet-friendly"
	case "cat":
		return "pet-friendly-cat"
	case "dog":
		return "pet-friendly-dog"
	case "both":
		return "pet-friendly-cat-or-dog"
	}
	return ""
}
