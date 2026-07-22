// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source auto
//
// divergence spends exactly ONE live request: the free and grossing charts both
// arrive in the same category_rankings response, so the join is done in memory.
//
// The spread is computed from EXACT RANKS ONLY. Revenue is carried for context
// but never feeds the arithmetic: the revenue figures are one-significant-figure
// buckets and cannot support a derived metric.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type divergenceRow struct {
	AppID    json.RawMessage `json:"app_id"`
	Name     string          `json:"name"`
	FreeRank int             `json:"free_rank"`
	// GrossingRank is null when the app does not appear on the grossing chart
	// at the depth fetched.
	GrossingRank *int `json:"grossing_rank"`
	// Spread is grossing_rank - free_rank. It is null exactly when
	// GrossingRank is null: there is no honest number to put there, and
	// inventing one would be the failure mode this command exists to avoid.
	Spread           *int    `json:"spread"`
	UnrankedGrossing bool    `json:"unranked_grossing"`
	Revenue          *string `json:"revenue"`
}

type divergenceResult struct {
	Category   string          `json:"category"`
	Country    string          `json:"country"`
	OS         string          `json:"os"`
	Date       string          `json:"date"`
	Divergence []divergenceRow `json:"divergence"`
	Note       string          `json:"note,omitempty"`
}

func newNovelDivergenceCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagOS string
	var flagDevice string
	var flagDate string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "divergence <category>",
		Short: "Find monetization outliers in a category by comparing each app's free-chart rank against its grossing-chart rank.",
		Long: "Join a category's free chart against its grossing chart and report the rank spread.\n\n" +
			"spread = grossing_rank - free_rank.\n" +
			"  Large positive: ranks high on installs but weak on revenue (installs without monetization).\n" +
			"  Large negative: monetizes well above its install volume.\n\n" +
			"Apps on the free chart but absent from the grossing chart at the fetched depth get\n" +
			"grossing_rank: null, spread: null, and unranked_grossing: true. The spread is derived\n" +
			"from exact ranks only — never from the bucketed revenue figures.",
		Example:     "  sensortower-pp-cli divergence 6015 --country US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch category rankings (1 request) and join the free chart against the grossing chart")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<category> is required (an iOS category id such as 6015, or an Android slug such as application)"))
			}
			category := args[0]
			osName, device, err := resolveOSDevice(flagOS, flagDevice)
			if err != nil {
				return err
			}
			date := flagDate
			if date == "" {
				date = defaultChartDate()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// One request; both charts come back in it.
			resp, err := fetchCategoryRankings(ctx, c, flags, osName, category, flagCountry, device, date, flagLimit)
			if err != nil {
				return err
			}

			grossingByID := make(map[string]int, len(resp.Data.Grossing))
			for _, g := range resp.Data.Grossing {
				if key := rawIDKey(g.AppID); key != "" {
					grossingByID[key] = g.Rank
				}
			}

			result := divergenceResult{
				Category:   category,
				Country:    flagCountry,
				OS:         osName,
				Date:       resp.Date,
				Divergence: make([]divergenceRow, 0, len(resp.Data.Free)),
			}
			unranked := 0
			for _, f := range resp.Data.Free {
				row := divergenceRow{
					AppID:    f.AppID,
					Name:     f.Name,
					FreeRank: f.Rank,
					Revenue:  revenueBucket(f.HumanizedRevenue),
				}
				if gr, ok := grossingByID[rawIDKey(f.AppID)]; ok {
					row.GrossingRank = intPtr(gr)
					row.Spread = intPtr(gr - f.Rank)
				} else {
					row.UnrankedGrossing = true
					unranked++
				}
				result.Divergence = append(result.Divergence, row)
			}

			// Sorted by spread desc. Rows with no grossing rank have no spread,
			// so they cannot be ordered against the ranked ones; they trail the
			// list rather than being assigned a fabricated extreme.
			sort.SliceStable(result.Divergence, func(i, j int) bool {
				a, b := result.Divergence[i], result.Divergence[j]
				if (a.Spread == nil) != (b.Spread == nil) {
					return b.Spread == nil
				}
				if a.Spread != nil && *a.Spread != *b.Spread {
					return *a.Spread > *b.Spread
				}
				return a.FreeRank < b.FreeRank
			})

			depth := resp.Limit
			if depth <= 0 {
				depth = flagLimit
			}
			if unranked > 0 {
				result.Note = fmt.Sprintf("%d of %d free-chart apps are absent from the top-%d grossing chart, so their spread is null (unranked_grossing): the data shows only that they rank below %d on grossing, not by how much. Revenue is a one-significant-figure bucket and never feeds the spread arithmetic.", unranked, len(result.Divergence), depth, depth)
			} else {
				result.Note = "spread is derived from exact chart ranks only; revenue is a one-significant-figure bucket shown for context and never feeds the arithmetic."
			}

			return emitNovelResult(cmd, flags, result, "divergence")
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code (e.g. US, GB, JP)")
	cmd.Flags().StringVar(&flagOS, "os", "ios", "Platform to chart (one of: ios, android)")
	cmd.Flags().StringVar(&flagDevice, "device", "", "Device chart variant; defaults to iphone on iOS and phone on Android")
	cmd.Flags().StringVar(&flagDate, "date", "", "Chart date as YYYY-MM-DD; defaults to today (UTC). The API rejects a blank date.")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Chart depth to fetch on both charts")
	return cmd
}
