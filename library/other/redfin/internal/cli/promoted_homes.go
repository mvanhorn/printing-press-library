// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// statusCodeFor maps user-friendly status enum values to Redfin's status int.
func statusCodeFor(s string) (int, error) {
	switch strings.ToLower(s) {
	case "for-sale", "active":
		return 1, nil
	case "sold":
		return 7, nil
	case "pending":
		return 9, nil
	case "coming-soon":
		return 130, nil
	case "":
		return 1, nil
	}
	return 0, fmt.Errorf("invalid --status %q (one of: for-sale, sold, pending, coming-soon)", s)
}

// uiPropertyTypesFor parses a comma-separated property-type list into Redfin's
// uipt int codes. Accepts: house, condo, townhouse, multi, manufactured, land.
func uiPropertyTypesFor(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	var out []int
	for _, raw := range strings.Split(s, ",") {
		t := strings.TrimSpace(strings.ToLower(raw))
		switch t {
		case "":
			continue
		case "house", "single-family":
			out = append(out, 1)
		case "condo":
			out = append(out, 2)
		case "townhouse":
			out = append(out, 3)
		case "multi", "multifamily":
			out = append(out, 4)
		case "manufactured":
			out = append(out, 5)
		case "land":
			out = append(out, 6)
		default:
			return nil, fmt.Errorf("unknown property type %q (one of: house, condo, townhouse, multi, manufactured, land)", raw)
		}
	}
	return out, nil
}

// homesFlags carries every filter the homes command exposes.
type homesFlags struct {
	regionID   int64
	regionType int
	regionSlug string
	status     string
	pType      string
	bedsMin    float64
	bathsMin   float64
	priceMin   int
	priceMax   int
	sqftMin    int
	sqftMax    int
	yearMin    int
	yearMax    int
	lotMin     int
	schoolsMin int
	polygon    string
	page       int
	limit      int
	all        bool
	sort       string
	// PATCH(upstream printing-press-library#482): --sold-window + --sf for Stingray sf-param control
	soldWindow string
	sf         string
}

// PATCH(upstream printing-press-library#482): new helper — Stingray's "Invalid
// arguments (code 101)" rejection of the prior hard-coded sf=1,3,5,7,9 default
// is fixed by mapping --sold-window through this function.
//
// validSoldWindows is the closed set of --sold-window values accepted.
// optsFromFlags validates against this before calling soldFlagsFor so
// typos like "1yr" surface as usage errors instead of silently
// resolving to the 3y default. Keep this set in sync with the cases
// in soldFlagsFor below — both must update together when a new window
// is added.
var validSoldWindows = map[string]bool{
	"1mo": true,
	"3mo": true,
	"6mo": true,
	"1y":  true,
	"2y":  true,
	"3y":  true,
}

// soldFlagsFor maps a CLI-facing --sold-window value to a Stingray
// "sf" parameter string. Stingray rejects ad-hoc multi-code unions
// with `Invalid arguments` (resultCode 101) — see issue #482 — so each
// returned value here is either a single bucket code (known-accepted)
// or the website's observed "include sold past 3 years" union
// (1,2,3,5,6,7). When users need a different window, --sf passes a raw
// value through; the default empty window resolves to the 3-year combo,
// which mirrors what redfin.com fires when you toggle "Sold" on the map.
//
// Callers should validate `window` against validSoldWindows before
// invoking this; the unknown-window fall-through here returns the 3y
// default defensively so the function is total (no panics on bad
// input), but optsFromFlags will refuse such input upstream so the
// fall-through never actually fires in production.
//
// Stingray sf bucket codes (from internal/cli/apt_comps.go:soldFlagsForMonths):
//
//	1=1mo  3=3mo  5=6mo  7=1y  9=2y
//
// Codes 2, 4, 6, 8 are observed in the website's 3y combo (1,2,3,5,6,7)
// but Stingray's docs don't expose their semantic meaning — likely
// "1mo–3mo span", "3mo–6mo span", etc. interstitial buckets the web UI
// includes when the user picks a multi-period window. Adding a new
// --sold-window value built from these requires capturing the matching
// combo from web traffic (network panel under the date filter) rather
// than guessing — Stingray rejects guessed unions.
func soldFlagsFor(window string) string {
	switch window {
	case "1mo":
		return "1"
	case "3mo":
		return "3"
	case "6mo":
		return "5"
	case "1y":
		return "7"
	case "2y":
		return "9"
	case "3y", "":
		// Website default for the "include sold past 3 years" filter
		// button — verified accepted against Stingray on 2026-05-12.
		return "1,2,3,5,6,7"
	}
	// Unknown explicit value — fall through to the verified 3y combo
	// rather than re-introducing the rejected 1,3,5,7,9 default.
	return "1,2,3,5,6,7"
}

// optsFromFlags builds a SearchOptions from the parsed flag struct, applying
// defaults (status=for-sale, limit=50, page=1) and validating enums.
func optsFromFlags(hf *homesFlags) (redfin.SearchOptions, error) {
	statusCode, err := statusCodeFor(hf.status)
	if err != nil {
		return redfin.SearchOptions{}, err
	}
	uipt, err := uiPropertyTypesFor(hf.pType)
	if err != nil {
		return redfin.SearchOptions{}, err
	}
	// PATCH(upstream printing-press-library#482): validate --sold-window
	// BEFORE region resolution so a typo surfaces even when the user
	// omits --region-slug/--region-id (which would otherwise short-
	// circuit with "region required"). --sf is a raw escape hatch and
	// bypasses this validation by design.
	if hf.sf == "" && hf.soldWindow != "" && !validSoldWindows[hf.soldWindow] {
		return redfin.SearchOptions{}, fmt.Errorf("invalid --sold-window %q (one of: 1mo|3mo|6mo|1y|2y|3y)", hf.soldWindow)
	}
	regionID := hf.regionID
	regionType := hf.regionType
	if hf.regionSlug != "" {
		id, typ, err := parseRegionSlug(hf.regionSlug)
		if err != nil {
			return redfin.SearchOptions{}, err
		}
		regionID = id
		regionType = typ
	}
	if regionID == 0 {
		return redfin.SearchOptions{}, usageErr(fmt.Errorf("region required: pass --region-id+--region-type or --region-slug"))
	}
	if regionType == 0 {
		regionType = 6
	}
	limit := hf.limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 350 {
		limit = 350
	}
	page := hf.page
	if page <= 0 {
		page = 1
	}
	soldFlags := ""
	if statusCode == 7 {
		// PATCH(upstream printing-press-library#482): replaced hard-coded
		// "1,3,5,7,9" (Stingray-rejected) with --sf-or-window resolution.
		// --sf <raw> wins (escape hatch for power users); else
		// --sold-window <name> maps to a known-valid code (validated at
		// the top of this function so typos surface even without a
		// region); else default to the website's 3y combo (1,2,3,5,6,7).
		switch {
		case hf.sf != "":
			soldFlags = hf.sf
		default:
			soldFlags = soldFlagsFor(hf.soldWindow)
		}
	}
	return redfin.SearchOptions{
		RegionID:        regionID,
		RegionType:      regionType,
		Status:          statusCode,
		SoldFlags:       soldFlags,
		UIPropertyTypes: uipt,
		BedsMin:         hf.bedsMin,
		BathsMin:        hf.bathsMin,
		PriceMin:        hf.priceMin,
		PriceMax:        hf.priceMax,
		SqftMin:         hf.sqftMin,
		SqftMax:         hf.sqftMax,
		YearMin:         hf.yearMin,
		YearMax:         hf.yearMax,
		LotMin:          hf.lotMin,
		SchoolsMin:      hf.schoolsMin,
		Polygon:         hf.polygon,
		NumHomes:        limit,
		PageNumber:      page,
		Sort:            hf.sort,
	}, nil
}

// printDryRunGet renders a 'would GET' line to stderr summarizing what the
// real call would send. Used by every command that wraps Stingray.
func printDryRunGet(cmd *cobra.Command, path string, params map[string]string) {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("would GET: ")
	b.WriteString(path)
	for i, k := range keys {
		if i == 0 {
			b.WriteString("?")
		} else {
			b.WriteString("&")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(params[k])
	}
	fmt.Fprintln(cmd.ErrOrStderr(), b.String())
}

// runHomesSearch shares the gis-search loop between the homes and sold
// commands. Returns the parsed listing rows; on --all, it walks up to 5 pages
// with a small adaptive delay.
func runHomesSearch(cmd *cobra.Command, flags *rootFlags, opts redfin.SearchOptions, all bool) ([]redfin.Listing, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}

	page := opts.PageNumber
	if page <= 0 {
		page = 1
	}
	maxPages := 1
	if all {
		maxPages = 5
	}

	var allListings []redfin.Listing
	for i := 0; i < maxPages; i++ {
		opts.PageNumber = page + i
		params := redfin.BuildSearchParams(opts)
		if i > 0 {
			time.Sleep(800 * time.Millisecond)
		}
		data, err := c.Get("/stingray/api/gis", params)
		if err != nil {
			return nil, classifyAPIError(err)
		}
		listings, perr := redfin.ParseSearchResponse(data)
		if perr != nil {
			// Pass back parser errors as API errors so users see what failed.
			fmt.Fprintf(os.Stderr, "warning: parse error on page %d: %v\n", opts.PageNumber, perr)
			break
		}
		allListings = append(allListings, listings...)
		if len(listings) < opts.NumHomes {
			break
		}
	}
	return allListings, nil
}

func newHomesCmd(flags *rootFlags) *cobra.Command {
	hf := &homesFlags{}

	cmd := &cobra.Command{
		Use:   "homes",
		Short: "Search Redfin listings via the Stingray gis endpoint with rich filtering.",
		Long: `Run a Stingray gis search and return parsed listing rows.

Region selection: pass either --region-id + --region-type, or --region-slug
(e.g. "city/30772/TX/Austin"). Numeric region IDs default to type=city.

Status maps user labels to Redfin codes: for-sale=1, sold=7, pending=9,
coming-soon=130. Property types map to Redfin's uipt codes:
house=1, condo=2, townhouse=3, multi=4, manufactured=5, land=6.

The Stingray response carries a literal {}&& CSRF prefix — that's
stripped automatically before parsing.`,
		Example: `  redfin-pp-cli homes --region-id 30772 --region-type 6 --beds-min 3 --price-max 600000 --status for-sale --json --limit 25
  redfin-pp-cli homes --region-slug "city/30772/TX/Austin" --beds-min 3 --json
  redfin-pp-cli homes --region-id 30772 --region-type 6 --status sold --year-min 2024 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, oerr := optsFromFlags(hf)
			if oerr != nil {
				if dryRunOK(flags) {
					// Dry-run still validates enums but tolerates missing region.
					if strings.Contains(oerr.Error(), "region required") {
						fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (region required at runtime)")
						return nil
					}
				}
				return oerr
			}
			if dryRunOK(flags) {
				printDryRunGet(cmd, "/stingray/api/gis", redfin.BuildSearchParams(opts))
				return nil
			}
			listings, err := runHomesSearch(cmd, flags, opts, hf.all)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), listings, flags)
		},
	}

	cmd.Flags().Int64Var(&hf.regionID, "region-id", 0, "Numeric Redfin region ID. Find via the URL slug or `region resolve`.")
	cmd.Flags().IntVar(&hf.regionType, "region-type", 6, "Region type: 1=zip, 2=state, 4=metro, 6=city, 11=neighborhood")
	cmd.Flags().StringVar(&hf.regionSlug, "region-slug", "", "Region slug like 'city/30772/TX/Austin' (alternative to --region-id+--region-type)")
	cmd.Flags().StringVar(&hf.status, "status", "for-sale", "Listing status: for-sale|sold|pending|coming-soon")
	cmd.Flags().StringVar(&hf.pType, "type", "", "Comma-separated property types: house,condo,townhouse,multi,manufactured,land")
	cmd.Flags().Float64Var(&hf.bedsMin, "beds-min", 0, "Minimum bedrooms")
	cmd.Flags().Float64Var(&hf.bathsMin, "baths-min", 0, "Minimum bathrooms")
	cmd.Flags().IntVar(&hf.priceMin, "price-min", 0, "Minimum price ($)")
	cmd.Flags().IntVar(&hf.priceMax, "price-max", 0, "Maximum price ($)")
	cmd.Flags().IntVar(&hf.sqftMin, "sqft-min", 0, "Minimum sqft")
	cmd.Flags().IntVar(&hf.sqftMax, "sqft-max", 0, "Maximum sqft")
	cmd.Flags().IntVar(&hf.yearMin, "year-min", 0, "Earliest year built")
	cmd.Flags().IntVar(&hf.yearMax, "year-max", 0, "Latest year built")
	cmd.Flags().IntVar(&hf.lotMin, "lot-min", 0, "Minimum lot size (sqft)")
	cmd.Flags().IntVar(&hf.schoolsMin, "schools-min", 0, "Minimum school rating (1-10)")
	cmd.Flags().StringVar(&hf.polygon, "polygon", "", "Bounding polygon: 'lat lng,lat lng,...'")
	cmd.Flags().IntVar(&hf.page, "page", 1, "1-indexed page number")
	cmd.Flags().IntVar(&hf.limit, "limit", 50, "Listings per page (max 350)")
	cmd.Flags().BoolVar(&hf.all, "all", false, "Auto-paginate up to 5 pages")
	cmd.Flags().StringVar(&hf.sort, "sort", "", "Sort: score-desc, price-asc, price-desc, days-on-redfin-asc")
	// PATCH(upstream printing-press-library#482): expose Stingray sf-param control.
	cmd.Flags().StringVar(&hf.soldWindow, "sold-window", "", "Sold-status time window: 1mo|3mo|6mo|1y|2y|3y (default: 3y). Ignored unless --status=sold.")
	cmd.Flags().StringVar(&hf.sf, "sf", "", "Raw Stingray 'sf' parameter (escape hatch; overrides --sold-window). Ignored unless --status=sold.")
	return cmd
}
