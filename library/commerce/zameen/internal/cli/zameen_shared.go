// Shared helpers for the hand-authored Zameen commands (find/pull/get/open and
// the novel watch/comps/deals/aging/agencies commands). Hand-authored.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/zameen"
)

// listingsResource is the resource_type key under which listings are stored.
const listingsResource = "listings"

// searchFlags holds the shared Zameen search filter flags.
type searchFlags struct {
	city         string
	location     string
	area         string
	purpose      string
	propertyType string
	minPrice     int
	maxPrice     int
	minBeds      int
	maxBeds      int
	minBaths     int
	minAreaMarla float64
	maxAreaMarla float64
	verifiedOnly bool
	sortKey      string
	limit        int
	maxScanPages int
}

// addSearchFlags registers the shared search filter flags on a command.
func addSearchFlags(cmd *cobra.Command, sf *searchFlags, withScan bool) {
	f := cmd.Flags()
	f.StringVar(&sf.city, "city", "", "City name (Islamabad, Lahore, Karachi, Rawalpindi, Faisalabad, Multan)")
	f.StringVar(&sf.location, "location", "", "Raw Zameen location slug with id (e.g. Islamabad-3, Lahore_DHA_Defence-9); overrides --city")
	f.StringVar(&sf.area, "area", "", "Filter to listings whose area/society matches this text (e.g. DHA_Defence, Bahria)")
	f.StringVar(&sf.purpose, "purpose", "buy", "buy or rent")
	f.StringVar(&sf.propertyType, "type", "Homes", "Property type: Homes, Plots, Commercial")
	f.IntVar(&sf.minPrice, "min-price", 0, "Minimum price in PKR")
	f.IntVar(&sf.maxPrice, "max-price", 0, "Maximum price in PKR")
	f.IntVar(&sf.minBeds, "min-beds", 0, "Minimum bedrooms")
	f.IntVar(&sf.maxBeds, "max-beds", 0, "Maximum bedrooms")
	f.IntVar(&sf.minBaths, "min-baths", 0, "Minimum bathrooms")
	f.Float64Var(&sf.minAreaMarla, "min-area", 0, "Minimum area in Marla")
	f.Float64Var(&sf.maxAreaMarla, "max-area", 0, "Maximum area in Marla")
	f.BoolVar(&sf.verifiedOnly, "verified", false, "Only Zameen-verified listings")
	f.StringVar(&sf.sortKey, "sort", "newest", "Sort: newest, price-asc, price-desc, area-asc, area-desc")
	f.IntVar(&sf.limit, "limit", 25, "Maximum matching listings to return")
	if withScan {
		f.IntVar(&sf.maxScanPages, "max-scan-pages", 5, "Maximum search pages to scan before returning (25 listings/page)")
	}
}

// toParams converts the shared flags to zameen.SearchParams, resolving the
// location slug and category segment. Returns a usage error on bad input.
func (sf *searchFlags) toParams() (zameen.SearchParams, error) {
	loc, err := zameen.ResolveLocation(sf.city, sf.location)
	if err != nil {
		return zameen.SearchParams{}, usageErr(err)
	}
	cat := zameen.ResolveCategory(sf.purpose, sf.propertyType)
	maxScan := sf.maxScanPages
	if maxScan <= 0 {
		maxScan = 5
	}
	return zameen.SearchParams{
		Category:     cat,
		Location:     loc,
		Area:         sf.area,
		PropertyType: sf.propertyType,
		Purpose:      zameen.NormalizePurpose(sf.purpose),
		MinPrice:     sf.minPrice,
		MaxPrice:     sf.maxPrice,
		MinBeds:      sf.minBeds,
		MaxBeds:      sf.maxBeds,
		MinBaths:     sf.minBaths,
		MinAreaMarla: sf.minAreaMarla,
		MaxAreaMarla: sf.maxAreaMarla,
		VerifiedOnly: sf.verifiedOnly,
		Sort:         sf.sortKey,
		Limit:        sf.limit,
		MaxScanPages: maxScan,
	}, nil
}

// emitListings writes a slice of listings honoring all output flags. A nil
// slice is emitted as an empty JSON array, never null.
func emitListings(cmd *cobra.Command, flags *rootFlags, listings []types.Listing) error {
	if listings == nil {
		listings = []types.Listing{}
	}
	raw, err := json.Marshal(listings)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}

// emitObject writes a single object (analytics view) honoring output flags.
func emitObject(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}

// loadStoredListings reads all synced listings from the local store.
func loadStoredListings(ctx context.Context, dbPath string) ([]types.Listing, error) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, errNoMirror
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	raws, err := db.List(listingsResource, 100000)
	if err != nil {
		return nil, fmt.Errorf("listing local listings: %w", err)
	}
	out := make([]types.Listing, 0, len(raws))
	for _, r := range raws {
		var l types.Listing
		if err := json.Unmarshal(r, &l); err != nil {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

// errNoMirror signals the local store has not been populated yet.
var errNoMirror = fmt.Errorf("no local mirror")

// emitEmptyMirrorHint prints the "run pull first" guidance for store-backed
// commands and returns nil (an empty local cache is not an error).
func emitEmptyMirrorHint(cmd *cobra.Command, flags *rootFlags, dbPath string) error {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no local mirror at %s\nrun: zameen-pp-cli pull --city <city> --purpose <buy|rent> --type <Homes|Plots|Commercial> --max-pages <n>\n",
		dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// filterStoredByArea keeps listings whose city/area/location matches the query.
func filterStoredByArea(listings []types.Listing, city, area string) []types.Listing {
	cityQ := strings.ToLower(strings.TrimSpace(city))
	areaQ := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(area), "_", " "))
	out := make([]types.Listing, 0, len(listings))
	for _, l := range listings {
		if cityQ != "" && !strings.Contains(strings.ToLower(l.City), cityQ) &&
			!strings.Contains(strings.ToLower(l.Location), cityQ) {
			continue
		}
		if areaQ != "" && !strings.Contains(strings.ToLower(l.Location), areaQ) &&
			!strings.Contains(strings.ToLower(l.City), areaQ) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// median returns the median of a sorted-or-unsorted int slice (0 if empty).
func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	s := append([]int(nil), vals...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// pricePerMarla returns price/area_marla, or 0 when area is missing.
func pricePerMarla(l types.Listing) float64 {
	if l.AreaMarla <= 0 {
		return 0
	}
	return float64(l.Price) / l.AreaMarla
}
