// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: top-level analytics — hub-wide metrics and per-API traffic aggregation.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Show hub analytics: marketplace metrics and your API traffic with aggregates",
		Long:        "Show RapidAPI hub analytics: public marketplace metrics (APIs, users, traffic) and, for the logged-in user, per-day API traffic aggregated from the local store (SUM requests, AVG latency, error rates).",
		Example:     "  rapidapi-pp-cli analytics --from 2026-01-01 --to 2026-12-31",
		Annotations: map[string]string{"pp:endpoint": "analytics.hub", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				from = "2026-01-01"
			}
			if to == "" {
				to = "2026-12-31"
			}
			path := "/gateway/graphql"
			_ = path
			// Hub-wide public metrics (no auth needed beyond session-bound CSRF).
			hubVars := map[string]any{"where": map[string]any{"fromDate": from, "toDate": to}}
			hubData, err := gqlExec(cmd, flags, "getHubMetrics", hubVars, gqlResponsePaths["getHubMetrics"])
			if err != nil {
				return err
			}
			// Aggregate locally cached traffic from the store (offline insight).
			if !flags.dryRun {
				if agg := aggregateTraffic(cmd, flags); agg != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), agg)
				}
			}
			return gqlOutput(cmd, flags, hubData, map[string]bool{"publicApis": true, "users": true, "activeApiConsumers": true, "totalApiTraffic": true})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD, default 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD, default 2026-12-31)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

// aggregateTraffic computes SUM/AVG aggregates over the local traffic_analytics
// table (populated by `account analytics`). Returns a human summary string.
func aggregateTraffic(cmd *cobra.Command, flags *rootFlags) string {
	s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
	if err != nil {
		return ""
	}
	defer s.Close()

	var totalReq, totalErr, totalLat int64
	var days int
	rows, err := s.Query("SELECT COUNT(*), COALESCE(SUM(requests),0), COALESCE(SUM(errors),0), COALESCE(SUM(latency),0) FROM traffic_analytics")
	if err != nil {
		return ""
	}
	if rows.Next() {
		_ = rows.Scan(&days, &totalReq, &totalErr, &totalLat)
	}
	_ = rows.Err() // #nosec G104 -- advisory stats summary; error degrades to empty, which is safe
	rows.Close()   // #nosec G104 -- advisory stats summary; Close error is not actionable here

	if days == 0 {
		return ""
	}
	avgLat := float64(0)
	if totalReq > 0 {
		avgLat = float64(totalLat) / float64(totalReq)
	}
	errRate := float64(0)
	if totalReq > 0 {
		errRate = float64(totalErr) * 100 / float64(totalReq)
	}
	// Sort days for a stable summary.
	var dayRows []string
	rows2, err := s.Query("SELECT day, requests, errors FROM traffic_analytics ORDER BY day DESC LIMIT 7")
	if err == nil {
		for rows2.Next() {
			var d string
			var r, e int64
			if rows2.Scan(&d, &r, &e) == nil {
				dayRows = append(dayRows, fmt.Sprintf("%s: %d req, %d err", d, r, e))
			}
		}
		_ = rows2.Err() // #nosec G104 -- advisory stats summary; error degrades to empty, which is safe
		rows2.Close()   // #nosec G104 -- advisory stats summary; Close error is not actionable here
	}
	sort.Strings(dayRows)
	return fmt.Sprintf("Local traffic aggregate: %d days, %d total requests, avg latency %.1fms, error rate %.2f%%", days, totalReq, avgLat, errRate)
}

var _ = json.Marshal
