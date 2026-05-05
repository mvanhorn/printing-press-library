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
		// Default sold-time filter window covering 1y/3y/5y so the gis call
		// doesn't return zero results when the user passes --status sold without
		// an explicit --sf.
		soldFlags = "1,3,5,7,9"
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
	return cmd
}
