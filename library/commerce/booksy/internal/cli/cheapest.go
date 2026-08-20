// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cheapest — rank nearby businesses by cheapest matching service.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelCheapestCmd(flags *rootFlags) *cobra.Command {
	var flagLocationID string
	var flagLocationGeo string
	var flagQuery string
	var flagService string
	var flagLimit int
	var flagMaxScan int

	cmd := &cobra.Command{
		Use:   "cheapest",
		Short: "Rank nearby businesses by the cheapest service matching your query (e.g. haircut)",
		Long: "Search a city, then fetch each business's services and rank them by the\n" +
			"cheapest service matching --service. Booksy never sorts by service price;\n" +
			"this joins search with per-business prices locally. Public — no token.\n" +
			"--max-scan bounds how many businesses are fetched in detail.",
		Example:     "  booksy-pp-cli cheapest --location-id 47905 --service haircut --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cheapest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			maxScan := flagMaxScan
			if cliutil.IsDogfoodEnv() && maxScan > 3 {
				maxScan = 3
			}

			// Search the city for candidate businesses.
			params := map[string]string{
				"query":               flagQuery,
				"response_type":       "listing_web",
				"sort_order":          "score",
				"businesses_per_page": strconv.Itoa(maxScan),
				"businesses_page":     "1",
			}
			if flagLocationID != "" {
				params["location_id"] = flagLocationID
			}
			if flagLocationGeo != "" {
				params["location_geo"] = flagLocationGeo
			}
			data, err := c.Get(ctx, "/core/v2/customer_api/businesses", params)
			if err != nil {
				return err
			}
			var searchEnv struct {
				Businesses []struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"businesses"`
			}
			if err := json.Unmarshal(data, &searchEnv); err != nil {
				return fmt.Errorf("parsing search: %w", err)
			}

			type rankRow struct {
				BusinessID   int64   `json:"business_id"`
				Name         string  `json:"name"`
				Rating       float64 `json:"rating"`
				ReviewsCount int     `json:"reviews_count"`
				Service      string  `json:"service"`
				Price        float64 `json:"price"`
				PriceLabel   string  `json:"price_label"`
				VariantID    int64   `json:"service_variant_id"`
			}
			rows := make([]rankRow, 0)
			failures := make([]bkFetchFailure, 0)
			scanned := 0
			for _, s := range searchEnv.Businesses {
				if scanned >= maxScan {
					break
				}
				scanned++
				biz, ferr := fetchBusiness(ctx, c, strconv.FormatInt(s.ID, 10))
				if ferr != nil {
					failures = append(failures, bkFetchFailure{BusinessID: strconv.FormatInt(s.ID, 10), Error: ferr.Error()})
					continue
				}
				best := cheapestMatching(biz, flagService)
				if best == nil {
					continue
				}
				rows = append(rows, rankRow{
					BusinessID:   biz.ID,
					Name:         biz.Name,
					Rating:       biz.ReviewsRank,
					ReviewsCount: biz.ReviewsCount,
					Service:      best.Service,
					Price:        best.Price,
					PriceLabel:   best.PriceLabel,
					VariantID:    best.VariantID,
				})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Price < rows[j].Price })
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d businesses failed to load and were skipped\n", len(failures))
			}

			view := struct {
				Service           string           `json:"service"`
				ScannedBusinesses int              `json:"scanned_businesses"`
				MaxScan           int              `json:"max_scan"`
				Count             int              `json:"count"`
				Businesses        []rankRow        `json:"businesses"`
				FetchFailures     []bkFetchFailure `json:"fetch_failures,omitempty"`
				Note              string           `json:"note,omitempty"`
			}{Service: flagService, ScannedBusinesses: scanned, MaxScan: maxScan, Count: len(rows), Businesses: rows, FetchFailures: failures}
			if len(rows) == 0 {
				view.Note = fmt.Sprintf("scanned %d businesses without finding a service matching %q; raise --max-scan or change --service/--query", scanned, flagService)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintf(out, "No business matching service %q found in %d scanned.\n", flagService, scanned)
				return nil
			}
			fmt.Fprintf(out, "Cheapest %q (scanned %d):\n", flagService, scanned)
			fmt.Fprintf(out, "%12s  %-34s  %6s  %-10s\n", "PRICE", "NAME", "RATING", "VARIANT")
			for _, r := range rows {
				name := r.Name
				if len([]rune(name)) > 34 {
					name = string([]rune(name)[:33]) + "…"
				}
				fmt.Fprintf(out, "%12s  %-34s  %6.2f  %-10d\n", r.PriceLabel, name, r.Rating, r.VariantID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagLocationID, "location-id", "", "Booksy city/area location id to scope the search")
	cmd.Flags().StringVar(&flagLocationGeo, "location-geo", "", "lat,lng to scope the search")
	cmd.Flags().StringVar(&flagQuery, "query", "barber", "Search query to find candidate businesses")
	cmd.Flags().StringVar(&flagService, "service", "haircut", "Service name to price and rank by (e.g. haircut)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max ranked results to return")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan", 10, "Max businesses to fetch in detail before ranking")
	return cmd
}
