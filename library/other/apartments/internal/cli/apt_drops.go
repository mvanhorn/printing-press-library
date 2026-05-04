// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// dropEntry describes one listing with a price drop within the window.
type dropEntry struct {
	URL          string  `json:"url"`
	EarliestRent int     `json:"earliest_rent"`
	LatestRent   int     `json:"latest_rent"`
	DropPct      float64 `json:"drop_pct"`
	ObservedAt   string  `json:"observed_at,omitempty"`
}

func newDropsCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var minPct float64
	var limit int

	cmd := &cobra.Command{
		Use:         "drops",
		Short:       "List synced listings whose max rent dropped by ≥N% within a time window.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli drops --since 14d --min-pct 5 --json
  apartments-pp-cli drops --since 30d --min-pct 10 --limit 50
  apartments-pp-cli drops --since 7d
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parseDurationLoose(sinceStr)
			if err != nil {
				return usageErr(err)
			}
			if window <= 0 {
				window = 14 * 24 * time.Hour
			}
			db, derr := openAptStore(cmd.Context())
			if derr != nil {
				return derr
			}
			defer db.Close()

			cutoff := time.Now().Add(-window).UTC()
			rows, err := db.DB().Query(
				`SELECT listing_url,
				        MAX(observed_at) AS latest_obs,
				        MIN(observed_at) AS earliest_obs
				 FROM listing_snapshots
				 WHERE observed_at >= ? AND max_rent IS NOT NULL AND max_rent > 0
				 GROUP BY listing_url
				 HAVING COUNT(*) >= 2`,
				cutoff.Format(time.RFC3339),
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			type pair struct {
				url        string
				latestTS   string
				earliestTS string
			}
			var pairs []pair
			for rows.Next() {
				var p pair
				if err := rows.Scan(&p.url, &p.latestTS, &p.earliestTS); err != nil {
					return err
				}
				pairs = append(pairs, p)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var out []dropEntry
			for _, p := range pairs {
				latestRow, err := singleSnapshot(db.DB().QueryRow(
					`SELECT max_rent FROM listing_snapshots
					 WHERE listing_url = ? AND observed_at = ? AND max_rent > 0
					 LIMIT 1`,
					p.url, p.latestTS,
				))
				if err != nil {
					continue
				}
				earliestRow, err := singleSnapshot(db.DB().QueryRow(
					`SELECT max_rent FROM listing_snapshots
					 WHERE listing_url = ? AND observed_at = ? AND max_rent > 0
					 LIMIT 1`,
					p.url, p.earliestTS,
				))
				if err != nil {
					continue
				}
				if earliestRow <= 0 || latestRow <= 0 {
					continue
				}
				dropPct := float64(earliestRow-latestRow) / float64(earliestRow) * 100.0
				if dropPct < minPct {
					continue
				}
				out = append(out, dropEntry{
					URL:          p.url,
					EarliestRent: earliestRow,
					LatestRent:   latestRow,
					DropPct:      dropPct,
					ObservedAt:   p.latestTS,
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].DropPct > out[j].DropPct
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if out == nil {
				out = []dropEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "14d", "Window: how far back to look (e.g. 7d, 24h).")
	cmd.Flags().Float64Var(&minPct, "min-pct", 5, "Minimum drop percentage.")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return.")
	return cmd
}

// singleSnapshot scans a single max_rent value from a *sql.Row.
func singleSnapshot(row interface {
	Scan(dest ...any) error
}) (int, error) {
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
