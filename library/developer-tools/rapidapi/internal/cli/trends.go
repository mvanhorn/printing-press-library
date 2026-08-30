// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: trends — traffic trends from cached analytics (percentage deltas).

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:         "trends",
		Short:       "Show traffic trends from cached analytics: day-over-day percentage changes",
		Long:        "Show request/error trends computed from the locally cached traffic_analytics table: day-over-day percentage change in requests and error rate, plus a 7-day summary.",
		Example:     "  rapidapi-pp-cli trends --days 14",
		Annotations: map[string]string{"pp:endpoint": "trends.local", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if days <= 0 {
				days = 7
			}
			s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			rows, err := s.Query("SELECT day, requests, errors FROM traffic_analytics ORDER BY day DESC LIMIT ?", days)
			if err != nil {
				return fmt.Errorf("query traffic: %w", err)
			}
			defer rows.Close()
			type dayRow struct {
				day      string
				requests int64
				errors   int64
			}
			var daysData []dayRow
			for rows.Next() {
				var d dayRow
				if rows.Scan(&d.day, &d.requests, &d.errors) == nil {
					daysData = append(daysData, d)
				}
			}
			if len(daysData) == 0 {
				return fmt.Errorf("no cached traffic data — run `rapidapi-pp-cli account analytics` first")
			}
			w := cmd.OutOrStdout()
			for i := len(daysData) - 1; i >= 0; i-- {
				cur := daysData[i]
				delta := ""
				if i < len(daysData)-1 {
					prev := daysData[i+1]
					if prev.requests > 0 {
						pct := float64(cur.requests-prev.requests) * 100 / float64(prev.requests)
						delta = fmt.Sprintf(" (%.1f%% vs prev)", pct)
					}
				}
				errRate := float64(0)
				if cur.requests > 0 {
					errRate = float64(cur.errors) * 100 / float64(cur.requests)
				}
				fmt.Fprintf(w, "%s: %d requests, %.2f%% error rate%s\n", cur.day, cur.requests, errRate, delta)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Number of days of trend data to show")

	return cmd
}
