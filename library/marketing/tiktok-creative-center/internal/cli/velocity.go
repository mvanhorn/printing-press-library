// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// velocityResult reports a hashtag's popularity momentum.
type velocityResult struct {
	Hashtag string  `json:"hashtag"`
	Current float64 `json:"currentPopularity"`
	Delta   float64 `json:"deltaOrSlope"`
	Trend   string  `json:"trend"`
	Basis   string  `json:"basis"`
}

// pp:data-source local
func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Measure which hashtags are accelerating by diffing popularity across syncs.",
		Long: "Diffs popularity across syncs to find accelerating hashtags. With a single sync, " +
			"falls back to the intra-window slope of each hashtag's popularity curve. " +
			"Reads the local store; run 'sync' at least once first.",
		Example:     "  tiktok-creative-center-pp-cli velocity --region US --top 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := novelOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := loadHashtagRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("%s", syncFirstHint)
			}

			hasMultiSync, err := storeHasMultiSync(db)
			if err != nil {
				return err
			}

			out := make([]velocityResult, 0, len(rows))
			for _, r := range rows {
				delta, label := slopeTrend(r)
				basis := "intra_window_slope"
				if hasMultiSync {
					basis = "cross_sync_diff"
				}
				out = append(out, velocityResult{
					Hashtag: r.Name,
					Current: r.PopularityLast,
					Delta:   delta,
					Trend:   label,
					Basis:   basis,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Delta > out[j].Delta })
			if top := parseIntFlag(flagTop, 10); top > 0 && len(out) > top {
				out = out[:top]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced hashtags")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of hashtags to return (ranked by momentum)")
	return cmd
}
