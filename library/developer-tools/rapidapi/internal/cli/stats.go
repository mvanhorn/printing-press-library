// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: stats — hub statistics with local store aggregates.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show hub usage statistics: traffic totals and cached-record counts",
		Long:        "Show RapidAPI hub usage statistics: total API traffic, users, consumers (live) plus counts and rates computed from the local store (cached APIs, categories, collections, traffic days).",
		Example:     "  rapidapi-pp-cli stats --from 2026-01-01 --to 2026-12-31",
		Annotations: map[string]string{"pp:endpoint": "stats.hub", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				from = "2026-01-01"
			}
			if to == "" {
				to = "2026-12-31"
			}
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"where": map[string]any{"fromDate": from, "toDate": to}}
			data, err := gqlExec(cmd, flags, "getHubMetrics", variables, gqlResponsePaths["getHubMetrics"])
			if err != nil {
				return err
			}
			// Local store aggregates (COUNT per resource type).
			if !flags.dryRun {
				if counts := cachedResourceCounts(cmd, flags); counts != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), counts)
				}
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"publicApis": true, "users": true, "activeApiConsumers": true, "totalApiTraffic": true})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD, default 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD, default 2026-12-31)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

// cachedResourceCounts reports COUNT(*) per cached resource type.
func cachedResourceCounts(cmd *cobra.Command, flags *rootFlags) string {
	s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
	if err != nil {
		return ""
	}
	defer s.Close()
	rows, err := s.Query("SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type ORDER BY COUNT(*) DESC")
	if err != nil {
		return ""
	}
	defer rows.Close()
	out := "Local cache: "
	first := true
	for rows.Next() {
		var rt string
		var n int64
		if rows.Scan(&rt, &n) == nil {
			if !first {
				out += ", "
			}
			out += fmt.Sprintf("%s=%d", rt, n)
			first = false
		}
	}
	if first {
		return "Local cache: empty (run a search first)"
	}
	return out
}
