// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: account analytics — API traffic analytics + local store aggregation.
//
// Custom polish-owned table: traffic_analytics (declared in the store migration
// below). The generated resource helpers do not cover analytics rows, so raw
// SQL is used per the printing-press novel-feature data-routing exception.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newAccountAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Show API traffic analytics for your APIs: requests, latency, errors over time",
		Long:        "Show API traffic analytics for the logged-in user's APIs: request volume, latency, and error counts per day over a date range. Results are cached in the local store (traffic_analytics table) and aggregated with SQL (SUM/GROUP BY) for offline re-querying.",
		Example:     "  rapidapi-pp-cli account analytics --from 2026-08-01 --to 2026-08-28",
		Annotations: map[string]string{"pp:endpoint": "account.analytics", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "--from=2026-08-01;--to=2026-08-28"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				from = "2026-01-01"
			}
			if to == "" {
				to = "2026-12-31"
			}
			variables := map[string]any{
				"where": map[string]any{"fromDate": from, "toDate": to},
			}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "apiTrafficAnalytics", variables, "data.apiTrafficAnalytics")
			if err != nil {
				return err
			}
			// Cache the traffic rows locally for offline aggregation.
			if !flags.dryRun {
				if err := cacheTrafficAnalytics(cmd, flags, data); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: caching analytics locally: %v\n", err)
				}
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"date": true, "requests": true, "latency": true, "errors": true})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD, default 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD, default 2026-12-31)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

// cacheTrafficAnalytics upserts the per-day traffic rows into the local
// traffic_analytics table. The table is created lazily via ensureTrafficTable.
func cacheTrafficAnalytics(cmd *cobra.Command, flags *rootFlags, data json.RawMessage) error {
	s, err := store.OpenWithContext(cmd.Context(), storePath(flags))
	if err != nil {
		return err
	}
	defer s.Close()

	if err := ensureTrafficTable(cmd, s); err != nil {
		return err
	}

	var rows []struct {
		Date     string `json:"date"`
		Requests int64  `json:"requests"`
		Latency  int64  `json:"latency"`
		Errors   int64  `json:"errors"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		if r.Date == "" {
			continue
		}
		if _, err := s.DB().ExecContext(cmd.Context(),
			`INSERT INTO traffic_analytics (day, requests, latency, errors)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(day) DO UPDATE SET requests=excluded.requests, latency=excluded.latency, errors=excluded.errors`,
			r.Date, r.Requests, r.Latency, r.Errors); err != nil {
			return err
		}
	}
	return nil
}

// ensureTrafficTable creates the custom analytics table if missing.
func ensureTrafficTable(cmd *cobra.Command, s *store.Store) error {
	_, err := s.DB().ExecContext(cmd.Context(), `
		CREATE TABLE IF NOT EXISTS traffic_analytics (
			day      TEXT PRIMARY KEY,
			requests INTEGER NOT NULL DEFAULT 0,
			latency  INTEGER NOT NULL DEFAULT 0,
			errors   INTEGER NOT NULL DEFAULT 0
		)`)
	return err
}

// storePath returns the CLI's store path (shared with the rest of the CLI).
func storePath(flags *rootFlags) string {
	// The generated CLI keeps its store under the data dir; reuse the same
	// resolution the learn-loop commands use.
	return learnDBPath("")
}
