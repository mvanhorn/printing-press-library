// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: live upstream search with honest match confidence.
//
// foodpanda's /search endpoint is fuzzy and never returns an empty result set —
// a nonsense query still returns rows. Without local scoring, weak matches are
// indistinguishable from real ones, so agents read noise as signal.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type fpFindRow struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Cuisine         string  `json:"cuisine"`
	MatchScore      int     `json:"match_score"`
	MatchConfidence string  `json:"match_confidence"`
	Rating          float64 `json:"rating"`
	DeliveryFee     float64 `json:"delivery_fee"`
	MinDeliveryTime int     `json:"minimum_delivery_time_min"`
	DistanceKm      float64 `json:"distance_km"`
}

type fpFindView struct {
	Query          string      `json:"query"`
	Results        []fpFindRow `json:"results"`
	StrongMatches  int         `json:"strong_matches"`
	WeakOrNone     int         `json:"weak_or_no_match"`
	ScannedVendors int         `json:"scanned_vendors"`
	AvailableCount int         `json:"available_count"`
	MaxScanPages   int         `json:"max_scan_pages"`
	SearchCaveat   string      `json:"search_caveat"`
	Note           string      `json:"note,omitempty"`
}

const fpSearchCaveat = "foodpanda's search endpoint is fuzzy and never returns an empty result set; " +
	"rows with match_confidence weak or none did not actually match the query."

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var (
		flagQuery    string
		lat, lng     float64
		limit        int
		maxScanPages int
		country      string
		vertical     string
		explain      bool
		minScore     int
	)

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search vendors live upstream and label how strongly each result actually matched the query.",
		Long: "Search foodpanda live and score how strongly each result actually matched.\n\n" +
			"Use this for live upstream search where match quality matters.\n" +
			"Do NOT use it for offline search over already-synced data; use 'search' for that.",
		Example:     "  foodpanda-pp-cli find --query sushi --latitude 31.5204 --longitude 74.3587 --explain --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--query=sushi;--latitude=31.5204;--longitude=74.3587;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "find")
			}
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				flagQuery = strings.TrimSpace(args[0])
			}
			if strings.TrimSpace(flagQuery) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required (positional or --query)"))
			}
			args = []string{flagQuery}
			if lat == 0 || lng == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--latitude and --longitude are required"))
			}
			query := strings.TrimSpace(args[0])

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sweep, err := fpSweep(ctx, c, fpDiscoSearch,
				fpQuery{Lat: lat, Lng: lng, Country: country, Vertical: vertical, Query: query}, maxScanPages)
			if err != nil {
				return err
			}

			view := fpFindView{
				Query: query, ScannedVendors: sweep.Scanned, AvailableCount: sweep.Available,
				MaxScanPages: sweep.MaxScanPage, SearchCaveat: fpSearchCaveat,
			}
			rows := make([]fpFindRow, 0, len(sweep.Vendors))
			for _, v := range sweep.Vendors {
				score := fpMatchScore(v, query)
				if score >= 50 {
					view.StrongMatches++
				} else {
					view.WeakOrNone++
				}
				if score < minScore {
					continue
				}
				rows = append(rows, fpFindRow{
					Code: v.Code, Name: v.Name, Cuisine: v.PrimaryCuisine(),
					MatchScore: score, MatchConfidence: fpMatchLabel(score),
					Rating: v.Rating, DeliveryFee: fpRound2(v.MinDeliveryFee),
					MinDeliveryTime: int(v.MinDeliveryTime), DistanceKm: fpRound2(v.Distance),
				})
			}
			// Strongest matches first; upstream order is not relevance-ordered.
			for i := 1; i < len(rows); i++ {
				for j := i; j > 0 && rows[j].MatchScore > rows[j-1].MatchScore; j-- {
					rows[j], rows[j-1] = rows[j-1], rows[j]
				}
			}
			view.Results = fpTrim(rows, limit)
			if len(view.Results) == 0 {
				view.Note = fmt.Sprintf("upstream returned %d rows for %q but none scored at or above --min-score %d; the endpoint never returns an empty set",
					sweep.Scanned, query, minScore)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%q — %d strong, %d weak/none of %d upstream rows\n",
				query, view.StrongMatches, view.WeakOrNone, sweep.Scanned)
			if explain {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", fpSearchCaveat)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			out := make([][]string, 0, len(view.Results))
			for _, r := range view.Results {
				row := []string{r.Code, truncate(r.Name, 32), truncate(r.Cuisine, 14),
					fmt.Sprintf("%.1f", r.Rating), fmt.Sprintf("%.0f", r.DeliveryFee)}
				if explain {
					row = append(row, fmt.Sprintf("%d", r.MatchScore), r.MatchConfidence)
				}
				out = append(out, row)
			}
			headers := []string{"CODE", "NAME", "CUISINE", "RATING", "FEE"}
			if explain {
				headers = append(headers, "SCORE", "MATCH")
			}
			return flags.printTable(cmd, headers, out)
		},
	}

	cmd.Flags().Float64Var(&lat, "latitude", 0, "Latitude of the delivery point (required)")
	cmd.Flags().Float64Var(&lng, "longitude", 0, "Longitude of the delivery point (required)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 2, "Maximum search pages to scan (48 results per page)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().StringVar(&vertical, "vertical", "restaurants", "Vendor vertical: restaurants or darkstores")
	cmd.Flags().BoolVar(&explain, "explain", false, "Show per-result match score and confidence")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Drop results scoring below this (0-100); use 50 to keep only real matches")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search text (alternative to the positional argument)")
	return cmd
}
