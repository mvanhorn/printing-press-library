// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelTractionCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string

	cmd := &cobra.Command{
		Use:         "traction",
		Short:       "Ranks your apps by recent rating and review volume, flagging rating drops",
		Long:        "traction ranks every app by its rating and review volume over a recent window and flags apps whose rating is trending down (newer reviews worse than older ones). Download counts are added when ASC_VENDOR_NUMBER is set (see the note in the output).",
		Example:     "  asc-pp-cli traction --window 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window := atoiOr(flagWindow, 30)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := c.Config.ASCPreflight(); err != nil {
				return err
			}
			ctx := cmd.Context()
			apps, err := fleetApps(ctx, c)
			if err != nil {
				return err
			}
			type row struct {
				App     string  `json:"app"`
				Rating  float64 `json:"rating"`
				Reviews int     `json:"reviews"`
				Trend   float64 `json:"trend"`
				Flag    string  `json:"flag"`
			}
			rows := []row{}
			for _, a := range apps {
				revs, _ := appReviews(ctx, c, a.ID, 150)
				revs = reviewsWithin(revs, window)
				if len(revs) == 0 {
					rows = append(rows, row{App: a.Name})
					continue
				}
				mean := meanRating(revs)
				trend := ratingTrend(revs)
				flag := ""
				if trend <= -0.3 {
					flag = "rating dropping"
				}
				rows = append(rows, row{a.Name, round1(mean), len(revs), round1(trend), flag})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Rating != rows[j].Rating {
					return rows[i].Rating > rows[j].Rating
				}
				return rows[i].Reviews > rows[j].Reviews
			})
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.App, fmt.Sprintf("%.1f", r.Rating),
					fmt.Sprintf("%d", r.Reviews), fmt.Sprintf("%+.1f", r.Trend), r.Flag})
			}
			if c.Config.ASCVendorNumber == "" && !flags.asJSON && !flags.quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: set ASC_VENDOR_NUMBER to add download counts to this leaderboard")
			}
			return renderFleet(cmd, flags, rows,
				[]string{"App", "Rating", "Reviews", "Trend", "Flag"}, table,
				"no apps found for this team")
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "", "Rating/review window in days (default 30)")
	return cmd
}
