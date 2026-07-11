// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelReviewsRecentCmd(flags *rootFlags) *cobra.Command {
	var flagLimit string
	var flagPerApp string

	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Newest written customer reviews across all your apps in one merged stream",
		Long:        "reviews recent merges every app's customer reviews into one stream sorted newest-first, so you can read the fleet's latest feedback in a single command instead of querying each app.",
		Example:     "  asc-pp-cli reviews recent --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			limit := atoiOr(flagLimit, 20)
			perApp := atoiOr(flagPerApp, 50)
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
				Date      string `json:"date"`
				App       string `json:"app"`
				Rating    int    `json:"rating"`
				Title     string `json:"title"`
				Body      string `json:"body"`
				Territory string `json:"territory"`
				Reviewer  string `json:"reviewer"`
			}
			rows := []row{}
			for _, a := range apps {
				revs, _ := appReviews(ctx, c, a.ID, perApp)
				for _, rev := range revs {
					rows = append(rows, row{rev.CreatedDate, a.Name, rev.Rating,
						rev.Title, truncate(rev.Body, 140), rev.Territory, rev.ReviewerNickname})
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
			if len(rows) > limit {
				rows = rows[:limit]
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{shortDate(r.Date), r.App, strconv.Itoa(r.Rating), dash(r.Title), r.Territory})
			}
			return renderFleet(cmd, flags, rows,
				[]string{"Date", "App", "Rating", "Title", "Territory"}, table,
				"no customer reviews yet")
		},
	}
	cmd.Flags().StringVar(&flagLimit, "limit", "", "Max reviews to show across all apps (default 20)")
	cmd.Flags().StringVar(&flagPerApp, "per-app", "", "Reviews to pull per app before merging (default 50)")
	return cmd
}
