// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newDomDistributionCmd(flags *rootFlags) *cobra.Command {
	var zip string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "dom-distribution",
		Short:       "Show the days-on-market distribution for active listings in a zip code",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Bucket active listings in a zip code by days-on-market:
  • Fresh   0–7 days   (just listed)
  • Recent  8–30 days  (normal velocity)
  • Stale   31–90 days (sitting, price cut likely)
  • Old     90+ days   (distressed or overpriced)

Useful for quickly reading market absorption rate from your local store.
Run 'redfin-pp-cli sync' first to populate data.`,
		Example: `  redfin-pp-cli dom-distribution --zip 94102
  redfin-pp-cli dom-distribution --zip 94103 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if zip == "" {
				return usageErr(fmt.Errorf("--zip is required"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `
				SELECT
					SUM(CASE WHEN CAST(json_extract(data, '$.dom.value') AS INTEGER) BETWEEN 0 AND 7 THEN 1 ELSE 0 END) AS fresh_0_7,
					SUM(CASE WHEN CAST(json_extract(data, '$.dom.value') AS INTEGER) BETWEEN 8 AND 30 THEN 1 ELSE 0 END) AS recent_8_30,
					SUM(CASE WHEN CAST(json_extract(data, '$.dom.value') AS INTEGER) BETWEEN 31 AND 90 THEN 1 ELSE 0 END) AS stale_31_90,
					SUM(CASE WHEN CAST(json_extract(data, '$.dom.value') AS INTEGER) > 90 THEN 1 ELSE 0 END) AS old_90_plus,
					COUNT(*) AS total,
					AVG(CAST(json_extract(data, '$.dom.value') AS REAL)) AS avg_dom,
					MIN(CAST(json_extract(data, '$.dom.value') AS INTEGER)) AS min_dom,
					MAX(CAST(json_extract(data, '$.dom.value') AS INTEGER)) AS max_dom
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.address.zip') = ?
				  AND json_extract(data, '$.status') = 1
				  AND json_extract(data, '$.dom.value') IS NOT NULL`

			rows, err := db.DB().QueryContext(cmd.Context(), query, zip)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", zip)
				}
				return fmt.Errorf("querying DOM distribution: %w", err)
			}
			defer rows.Close()

			if !rows.Next() {
				fmt.Fprintf(cmd.OutOrStdout(), "No active listings with DOM data found for zip %s.\n", zip)
				return nil
			}

			var fresh, recent, stale, old, total int
			var minDOM, maxDOM int
			var avgDOM *float64
			if err := rows.Scan(&fresh, &recent, &stale, &old, &total, &avgDOM, &minDOM, &maxDOM); err != nil {
				return fmt.Errorf("scanning result: %w", err)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			pct := func(n int) float64 {
				if total == 0 {
					return 0
				}
				return roundTo(float64(n)/float64(total)*100, 1)
			}

			type Bucket struct {
				Label   string  `json:"label"`
				Range   string  `json:"range_days"`
				Count   int     `json:"count"`
				Percent float64 `json:"percent"`
			}

			type Distribution struct {
				Zip      string   `json:"zip"`
				Total    int      `json:"total_active"`
				MinDOM   int      `json:"min_dom"`
				MaxDOM   int      `json:"max_dom"`
				AvgDOM   float64  `json:"avg_dom"`
				Buckets  []Bucket `json:"buckets"`
				Signal   string   `json:"market_signal"`
			}

			avgDOMVal := 0.0
			if avgDOM != nil {
				avgDOMVal = roundTo(*avgDOM, 1)
			}

			signal := ""
			switch {
			case pct(fresh) >= 40:
				signal = "high absorption — lots of fresh listings, fast market"
			case pct(old) >= 30:
				signal = "low absorption — significant old inventory, buyer's market"
			case pct(stale) >= 40:
				signal = "slowing — stale inventory building, price cuts likely"
			default:
				signal = "balanced — normal DOM distribution"
			}

			dist := Distribution{
				Zip:    zip,
				Total:  total,
				MinDOM: minDOM,
				MaxDOM: maxDOM,
				AvgDOM: avgDOMVal,
				Buckets: []Bucket{
					{Label: "Fresh", Range: "0–7", Count: fresh, Percent: pct(fresh)},
					{Label: "Recent", Range: "8–30", Count: recent, Percent: pct(recent)},
					{Label: "Stale", Range: "31–90", Count: stale, Percent: pct(stale)},
					{Label: "Old", Range: "90+", Count: old, Percent: pct(old)},
				},
				Signal: signal,
			}

			return printJSONFiltered(cmd.OutOrStdout(), dist, flags)
		},
	}

	cmd.Flags().StringVar(&zip, "zip", "", "Zip code to analyze (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
