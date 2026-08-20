// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type netInvestmentStreakView struct {
	Asset            string `json:"asset"`
	CurrentDirection string `json:"current_direction"` // buying | selling | flat
	CurrentStreak    int    `json:"current_streak_periods"`
	StreakStart      string `json:"streak_start_period"`
	StreakEnd        string `json:"streak_end_period"`
	LastFlipPeriod   string `json:"last_flip_period,omitempty"`
	PeriodsAnalyzed  int    `json:"periods_analyzed"`
	Note             string `json:"note,omitempty"`
}

func newNovelNetInvestmentStreaksCmd(flags *rootFlags) *cobra.Command {
	var flagAsset string

	cmd := &cobra.Command{
		Use:         "streaks",
		Short:       "See how many consecutive fortnights/years FPIs have been net buyers or sellers, and the most recent flip.",
		Example:     "  fpi-india-pp-cli net-investment streaks --asset equity --json",
		Long:        "Walks the synced financial-year net-investment series in order and reports the current consecutive buying/selling streak and the most recent sign flip. Requires 'sync --resources net_investment' first.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute buying/selling streak")
				return nil
			}
			asset := flagAsset
			if asset == "" {
				asset = "total"
			}

			db, exists, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: fpi-india-pp-cli sync --resources net_investment\n", defaultDBPath("fpi-india-pp-cli"))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			defer db.Close()

			rows, err := loadNetInvestmentRows(db, "fy", "INR")
			if err != nil {
				return fmt.Errorf("reading synced net-investment data: %w", err)
			}
			if len(rows) == 0 {
				view := netInvestmentStreakView{Asset: asset, Note: "no synced net-investment rows; run sync --resources net_investment --full"}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}

			type point struct {
				period string
				value  float64
			}
			var points []point
			for _, r := range rows {
				v, ok := r.AssetValue(asset)
				if !ok {
					_ = cmd.Usage()
					return usageErrf("--asset must be one of equity, debt, hybrid, mutual_funds, aif, total")
				}
				if v == 0 {
					continue // skip pre-instrument-existence zero rows (e.g. AIF before it existed)
				}
				points = append(points, point{r.Period, v})
			}
			if len(points) == 0 {
				view := netInvestmentStreakView{Asset: asset, Note: "no non-zero periods found for this asset class"}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}

			dirOf := func(v float64) string {
				if v > 0 {
					return "buying"
				}
				return "selling"
			}

			currentDir := dirOf(points[len(points)-1].value)
			streak := 1
			streakStartIdx := len(points) - 1
			for i := len(points) - 2; i >= 0; i-- {
				if dirOf(points[i].value) != currentDir {
					break
				}
				streak++
				streakStartIdx = i
			}
			lastFlip := ""
			if streakStartIdx > 0 {
				lastFlip = points[streakStartIdx-1].period
			}

			view := netInvestmentStreakView{
				Asset:            asset,
				CurrentDirection: currentDir,
				CurrentStreak:    streak,
				StreakStart:      points[streakStartIdx].period,
				StreakEnd:        points[len(points)-1].period,
				LastFlipPeriod:   lastFlip,
				PeriodsAnalyzed:  len(points),
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d consecutive %s periods (%s to %s)\n",
				asset, view.CurrentStreak, view.CurrentDirection, view.StreakStart, view.StreakEnd)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAsset, "asset", "", "Asset class: equity, debt, hybrid, mutual_funds, aif, or total (default total)")
	return cmd
}
