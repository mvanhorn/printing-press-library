// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: which vendors actually reach a point, and how that differs
// from a second point. The listing endpoint is itself the coverage oracle —
// it returns only vendors that deliver to the queried coordinate — so a set
// difference across two sweeps answers a question the UI cannot express.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type fpCoverageRow struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Cuisine         string  `json:"cuisine"`
	DeliveryFee     float64 `json:"delivery_fee"`
	MinDeliveryTime int     `json:"minimum_delivery_time_min"`
	DistanceKm      float64 `json:"distance_km"`
	Rating          float64 `json:"rating"`
}

type fpCoverageView struct {
	Target         fpPoint         `json:"target"`
	CompareTo      *fpPoint        `json:"compare_to,omitempty"`
	Serving        []fpCoverageRow `json:"serving_target"`
	OnlyTarget     []fpCoverageRow `json:"only_at_target,omitempty"`
	OnlyCompare    []fpCoverageRow `json:"only_at_compare,omitempty"`
	ServingCount   int             `json:"serving_count"`
	ScannedVendors int             `json:"scanned_vendors"`
	AvailableCount int             `json:"available_count"`
	MaxScanPages   int             `json:"max_scan_pages"`
	ScanCapHit     bool            `json:"scan_cap_hit"`
	Note           string          `json:"note,omitempty"`
}

type fpPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng       float64
		cmpLat, cmpLng float64
		limit          int
		maxScanPages   int
		country        string
	)

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Determine which vendors actually deliver to an arbitrary point using each vendor's published delivery radius.",
		Long: "Show which vendors actually deliver to a coordinate, and optionally how that set\n" +
			"differs from a second coordinate.\n\n" +
			"Use this before assuming a restaurant is orderable from an address you have not tried,\n" +
			"or to see what moving location would gain and lose.\n" +
			"Do NOT use it to compare prices; use 'fees' for that.",
		Example:     "  foodpanda-pp-cli coverage --latitude 31.4820 --longitude 74.3430 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			if lat == 0 || lng == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--latitude and --longitude are required"))
			}
			if (cmpLat == 0) != (cmpLng == 0) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--compare-latitude and --compare-longitude must be given together"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			target, err := fpSweep(ctx, c, fpDiscoVendors, fpQuery{Lat: lat, Lng: lng, Country: country}, maxScanPages)
			if err != nil {
				return err
			}

			view := fpCoverageView{
				Target: fpPoint{Latitude: lat, Longitude: lng}, ServingCount: len(target.Vendors),
				ScannedVendors: target.Scanned, AvailableCount: target.Available,
				MaxScanPages: target.MaxScanPage, ScanCapHit: target.ScanCapHit,
			}
			view.Serving = coverageRows(fpTrim(target.Vendors, limit))

			if cmpLat != 0 && cmpLng != 0 {
				other, err := fpSweep(ctx, c, fpDiscoVendors, fpQuery{Lat: cmpLat, Lng: cmpLng, Country: country}, maxScanPages)
				if err != nil {
					return err
				}
				view.CompareTo = &fpPoint{Latitude: cmpLat, Longitude: cmpLng}
				view.ScannedVendors += other.Scanned

				inTarget := map[string]fpVendor{}
				for _, v := range target.Vendors {
					inTarget[v.Code] = v
				}
				inOther := map[string]fpVendor{}
				for _, v := range other.Vendors {
					inOther[v.Code] = v
				}
				var onlyT, onlyO []fpVendor
				for _, v := range target.Vendors {
					if _, ok := inOther[v.Code]; !ok {
						onlyT = append(onlyT, v)
					}
				}
				for _, v := range other.Vendors {
					if _, ok := inTarget[v.Code]; !ok {
						onlyO = append(onlyO, v)
					}
				}
				view.OnlyTarget = coverageRows(fpTrim(onlyT, limit))
				view.OnlyCompare = coverageRows(fpTrim(onlyO, limit))
			}

			// The scan cap truncates silently: at a dense coordinate the sweep
			// stops after --max-scan-pages and reports only what it saw, so an
			// unqualified "N vendors deliver here" understates coverage by
			// however much was never fetched. Disclose the cap whenever it is
			// hit, not only when the result set came back empty.
			switch {
			case len(view.Serving) == 0 && target.ScanCapHit:
				view.Note = fmt.Sprintf("scanned %d vendors across %d page(s) without finding coverage; raise --max-scan-pages to widen the search",
					target.Scanned, target.PagesRead)
			case target.ScanCapHit:
				view.Note = fmt.Sprintf("partial coverage: stopped at the --max-scan-pages limit of %d page(s) after %d of %d vendors available at this point; raise --max-scan-pages for the full set",
					target.PagesRead, target.Scanned, target.Available)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Serving) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No vendors deliver to %.5f,%.5f.\n", lat, lng)
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			if view.ScanCapHit {
				fmt.Fprintf(cmd.OutOrStdout(), "%d of %d vendors deliver to %.5f,%.5f (partial — scan capped)\n",
					view.ServingCount, view.AvailableCount, lat, lng)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%d vendors deliver to %.5f,%.5f\n", view.ServingCount, lat, lng)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+view.Note)
			}
			if view.CompareTo != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  gained vs compare point: %d   lost vs compare point: %d\n",
					len(view.OnlyTarget), len(view.OnlyCompare))
			}
			fmt.Fprintln(cmd.OutOrStdout())
			out := make([][]string, 0, len(view.Serving))
			for _, r := range view.Serving {
				out = append(out, []string{
					r.Code, truncate(r.Name, 32), truncate(r.Cuisine, 14),
					fmt.Sprintf("%.0f", r.DeliveryFee), fmt.Sprintf("%d", r.MinDeliveryTime),
					fmt.Sprintf("%.1f", r.DistanceKm), fmt.Sprintf("%.1f", r.Rating),
				})
			}
			return flags.printTable(cmd, []string{"CODE", "NAME", "CUISINE", "FEE", "MINS", "KM", "RATING"}, out)
		},
	}

	cmd.Flags().Float64Var(&lat, "latitude", 0, "Latitude of the point to test (required)")
	cmd.Flags().Float64Var(&lng, "longitude", 0, "Longitude of the point to test (required)")
	cmd.Flags().Float64Var(&cmpLat, "compare-latitude", 0, "Optional second latitude to diff coverage against")
	cmd.Flags().Float64Var(&cmpLng, "compare-longitude", 0, "Optional second longitude to diff coverage against")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum vendors to list per section")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 4, "Maximum listing pages to scan per point (48 vendors per page)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	return cmd
}

func coverageRows(vs []fpVendor) []fpCoverageRow {
	rows := make([]fpCoverageRow, 0, len(vs))
	for _, v := range vs {
		rows = append(rows, fpCoverageRow{
			Code: v.Code, Name: v.Name, Cuisine: v.PrimaryCuisine(),
			DeliveryFee: fpRound2(v.MinDeliveryFee), MinDeliveryTime: int(v.MinDeliveryTime),
			DistanceKm: fpRound2(v.Distance), Rating: v.Rating,
		})
	}
	return rows
}
