// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: rank restaurants near your saved address by real delivery cost.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type fpHomeRow struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Cuisine         string  `json:"cuisine"`
	DeliveryFee     float64 `json:"delivery_fee"`
	MinOrderAmount  float64 `json:"minimum_order_amount"`
	MinDeliveryTime int     `json:"minimum_delivery_time_min"`
	Rating          float64 `json:"rating"`
	ReviewCount     int     `json:"review_count"`
	DistanceKm      float64 `json:"distance_km"`
	IsDeliveryOpen  bool    `json:"is_delivery_enabled"`
	FeeSource       string  `json:"fee_source"`
}

type fpHomeView struct {
	Address                string      `json:"address"`
	City                   string      `json:"city"`
	Country                string      `json:"country"`
	Vendors                []fpHomeRow `json:"vendors"`
	ScannedVendors         int         `json:"scanned_vendors"`
	AvailableCount         int         `json:"available_count"`
	MaxScanPages           int         `json:"max_scan_pages"`
	DeliveryFeePricedCount int         `json:"delivery_fee_priced_count"`
	DeliveryFeeTotal       int         `json:"delivery_fee_vendor_count"`
	FeeLookupFailures      []string    `json:"fee_lookup_failures,omitempty"`
	Note                   string      `json:"note,omitempty"`
}

func newNovelHomeCmd(flags *rootFlags) *cobra.Command {
	var (
		sortKey      string
		limit        int
		maxScanPages int
		label        string
		country      string
		vertical     string
		openOnly     bool
		noRealFees   bool
		concurrency  int
		lat, lng     float64
	)

	cmd := &cobra.Command{
		Use:   "home",
		Short: "Rank every restaurant near your saved home address by what delivery actually costs you.",
		Long: "Rank every restaurant that reaches your saved foodpanda address by real delivery cost.\n\n" +
			"Use this command when the question is what is cheapest to get delivered to you.\n" +
			"Do NOT use it to look up one known restaurant; use 'menu <vendor_code>' instead.\n" +
			"Requires a session: run 'foodpanda-pp-cli auth login --chrome' first.",
		Example:     "  foodpanda-pp-cli home --sort fee --limit 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--latitude=31.5204;--longitude=74.3587;--limit=5;--no-real-fees"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "home")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Explicit coordinates preview any location without a session; the
			// saved address remains the default when none are given.
			var addr fpAddress
			if lat != 0 && lng != 0 {
				addr = fpAddress{Latitude: lat, Longitude: lng, Label: fmt.Sprintf("%.5f,%.5f", lat, lng)}
			} else {
				addrs, ferr := fpFetchAddresses(ctx, c)
				if ferr != nil {
					return authErr(fmt.Errorf("could not read saved addresses (run 'foodpanda-pp-cli auth login --chrome', or pass --latitude/--longitude): %w", ferr))
				}
				addr, ferr = fpPickAddress(addrs, label)
				if ferr != nil {
					return usageErr(ferr)
				}
			}

			q := fpQuery{Lat: addr.Latitude, Lng: addr.Longitude, Country: country, Vertical: vertical}
			sweep, err := fpSweep(ctx, c, fpDiscoVendors, q, maxScanPages)
			if err != nil {
				return err
			}

			vendors := sweep.Vendors
			if openOnly {
				kept := make([]fpVendor, 0, len(vendors))
				for _, v := range vendors {
					if v.IsDeliveryEnabled && v.IsActive {
						kept = append(kept, v)
					}
				}
				vendors = kept
			}
			fpSortVendors(vendors, sortKey)
			vendors = fpTrim(vendors, limit)

			// The listing fee is a flat floor (99 everywhere in Lahore). The true
			// per-vendor fee only exists on the vendor-detail endpoint with an
			// authenticated session, so resolve it for the vendors we return.
			realFees := map[string]fpVendorFee{}
			if !noRealFees && len(vendors) > 0 {
				codes := make([]string, 0, len(vendors))
				for _, v := range vendors {
					codes = append(codes, v.Code)
				}
				realFees = fpFetchVendorFees(ctx, c, q.Country, codes, concurrency)
			}

			feeErrs := make([]string, 0)
			rows := make([]fpHomeRow, 0, len(vendors))
			for _, v := range vendors {
				fee, minOrder, src := v.MinDeliveryFee, v.MinOrderAmount, "listing-floor"
				if rf, ok := realFees[v.Code]; ok {
					switch {
					case rf.Err != nil:
						src = "lookup-failed"
						feeErrs = append(feeErrs, fmt.Sprintf("%s (%s): %s", v.Name, v.Code, truncate(rf.Err.Error(), 160)))
					case rf.Resolved:
						fee, src = rf.DeliveryFee, "vendor-detail"
						if rf.MinOrder > 0 {
							minOrder = rf.MinOrder
						}
					default:
						src = "unpriced"
					}
				}
				rows = append(rows, fpHomeRow{
					Code: v.Code, Name: v.Name, Cuisine: v.PrimaryCuisine(),
					DeliveryFee: fpRound2(fee), MinOrderAmount: fpRound2(minOrder),
					MinDeliveryTime: int(v.MinDeliveryTime), Rating: v.Rating, ReviewCount: int(v.ReviewNumber),
					DistanceKm: fpRound2(v.Distance), IsDeliveryOpen: v.IsDeliveryEnabled,
					FeeSource: src,
				})
			}

			if sortKey == "fee" {
				sort.SliceStable(rows, func(i, j int) bool { return rows[i].DeliveryFee < rows[j].DeliveryFee })
			}

			view := fpHomeView{
				Address: addr.Describe(), City: addr.City, Country: q.Country,
				Vendors: rows, ScannedVendors: sweep.Scanned, AvailableCount: sweep.Available,
				MaxScanPages: sweep.MaxScanPage,
			}
			pricedN, totalN := 0, len(rows)
			for _, r := range rows {
				if r.FeeSource == "vendor-detail" {
					pricedN++
				}
			}
			feeMissing := totalN > 0 && pricedN*5 < totalN
			feeNote := ""
			if feeMissing {
				feeNote = fmt.Sprintf("only %d of %d vendors returned a real per-vendor delivery fee; the rest fall back to "+
					"foodpanda's flat listing floor. Run 'foodpanda-pp-cli auth login --chrome' for session-priced fees.", pricedN, totalN)
				view.Note = feeNote
			}
			view.DeliveryFeePricedCount, view.DeliveryFeeTotal = pricedN, totalN
			if len(feeErrs) > 0 {
				view.FeeLookupFailures = feeErrs
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d per-vendor fee lookups failed; those rows fall back to the listing floor\n  %s\n",
					len(feeErrs), totalN, feeErrs[0])
			}
			if len(rows) == 0 && sweep.ScanCapHit {
				view.Note = fmt.Sprintf("scanned %d vendors across %d page(s) without a match; raise --max-scan-pages to widen the search",
					sweep.Scanned, sweep.PagesRead)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No restaurants found for %s.\n", addr.Describe())
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Delivering to %s, %s  (%d of %d vendors scanned)\n",
				addr.Describe(), addr.City, sweep.Scanned, sweep.Available)
			if feeMissing {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", feeNote)
				fmt.Fprintf(cmd.OutOrStdout(), "note: only %d of %d vendors are delivery-fee priced; 0 means unpriced, not free\n", pricedN, totalN)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Code, truncate(r.Name, 34), truncate(r.Cuisine, 14),
					fmt.Sprintf("%.0f", r.DeliveryFee), fmt.Sprintf("%.0f", r.MinOrderAmount),
					fmt.Sprintf("%d", r.MinDeliveryTime), fmt.Sprintf("%.1f", r.Rating),
				})
			}
			return flags.printTable(cmd, []string{"CODE", "NAME", "CUISINE", "FEE", "MIN ORDER", "MINS", "RATING"}, out)
		},
	}

	cmd.Flags().StringVar(&sortKey, "sort", "fee", "Sort by: fee, rating, distance, time, min-order, name")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum vendors to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 4, "Maximum listing pages to scan (48 vendors per page)")
	cmd.Flags().Float64Var(&lat, "latitude", 0, "Preview a specific coordinate instead of your saved address")
	cmd.Flags().Float64Var(&lng, "longitude", 0, "Preview a specific coordinate instead of your saved address")
	cmd.Flags().StringVar(&label, "address", "", "Saved address label or street fragment to use (default: your default address)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().StringVar(&vertical, "vertical", "restaurants", "Vendor vertical: restaurants or darkstores")
	cmd.Flags().BoolVar(&noRealFees, "no-real-fees", false, "Skip per-vendor fee lookups and report only the flat listing floor (faster, less accurate)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 6, "Parallel per-vendor fee lookups")
	cmd.Flags().BoolVar(&openOnly, "open-only", false, "Only include vendors currently accepting delivery")
	return cmd
}
