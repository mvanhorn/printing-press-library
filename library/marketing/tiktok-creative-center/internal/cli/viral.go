// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// viralResult is one row of the opportunity ranking.
type viralResult struct {
	Hashtag          string  `json:"hashtag"`
	PublishCnt       float64 `json:"publishCnt"`
	Popularity       float64 `json:"popularity"`
	OpportunityScore float64 `json:"opportunityScore"`
	Rank             int     `json:"rank"`
}

// pp:data-source local
func newNovelViralCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagDays string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "viral",
		Short: "Rank hashtags by opportunity = high popularity + low publish count (underserved but rising).",
		Long: "Rank hashtags by an opportunity score = popularity / publish count so you can find what is " +
			"rising but not yet saturated — the signal for what to create before everyone piles in. " +
			"Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli viral --region US --days 7 --top 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			_ = flagDays // days already applied at sync time; accepted for symmetry
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

			ranked := viralRank(rows, parseIntFlag(flagTop, 10))
			out := make([]viralResult, 0, len(ranked))
			for i, r := range ranked {
				out = append(out, viralResult{
					Hashtag:          r.Name,
					PublishCnt:       r.PublishCnt,
					Popularity:       r.Popularity,
					OpportunityScore: opportunityScore(r),
					Rank:             i + 1,
				})
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced hashtags")
	cmd.Flags().StringVar(&flagDays, "days", "7", "Time range in days (informational; applied at sync time)")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of top opportunities to return")
	return cmd
}
