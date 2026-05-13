// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newPriceTrendCmd(flags *rootFlags) *cobra.Command {
	var zip string
	var weeks int
	var field string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "price-trend",
		Short:       "Show price, DOM, and inventory as a weekly time series for a zip code",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Group locally synced listings by week and compute median price, median days-on-market,
and inventory count per time bucket. Requires multiple syncs over time to show trends.

Run 'redfin-pp-cli sync' periodically to accumulate history.`,
		Example: `  # Weekly median price for zip 94102 over 12 weeks
  redfin-pp-cli price-trend --zip 94102

  # Show DOM trend instead
  redfin-pp-cli price-trend --zip 94102 --field dom

  # JSON output for agent charting
  redfin-pp-cli price-trend --zip 94102 --weeks 24 --json`,
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

			// Validate field
			validFields := map[string]bool{"median_price": true, "dom": true, "inventory": true}
			if field != "" && !validFields[field] {
				return usageErr(fmt.Errorf("--field must be one of: median_price, dom, inventory"))
			}

			// Query: group by week using synced_at, compute aggregates
			query := `
				SELECT
					strftime('%Y-%W', synced_at) AS week,
					COUNT(*) AS inventory,
					AVG(CAST(json_extract(data, '$.price.value') AS REAL)) AS avg_price,
					AVG(CAST(json_extract(data, '$.dom.value') AS REAL)) AS avg_dom
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.address.zip') = ?
				  AND synced_at >= datetime('now', ?)
				GROUP BY week
				ORDER BY week ASC`

			weekParam := fmt.Sprintf("-%d weeks", weeks)
			rows, err := db.DB().QueryContext(cmd.Context(), query, zip, weekParam)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", zip)
				}
				return fmt.Errorf("querying price trend: %w", err)
			}
			defer rows.Close()

			type TrendPoint struct {
				Week      string  `json:"week"`
				Inventory int     `json:"inventory"`
				AvgPrice  float64 `json:"avg_price"`
				AvgDOM    float64 `json:"avg_dom"`
			}

			var points []TrendPoint
			for rows.Next() {
				var p TrendPoint
				var avgPrice, avgDOM *float64
				if err := rows.Scan(&p.Week, &p.Inventory, &avgPrice, &avgDOM); err != nil {
					continue
				}
				if avgPrice != nil {
					p.AvgPrice = roundTo((*avgPrice), 0)
				}
				if avgDOM != nil {
					p.AvgDOM = roundTo(*avgDOM, 1)
				}
				points = append(points, p)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(points) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No trend data for zip %s — sync more data over time to build a time series.\n", zip)
				return nil
			}

			// If --field specified, project to simpler output
			if field != "" {
				type SimplePoint struct {
					Week  string  `json:"week"`
					Value float64 `json:"value"`
					Field string  `json:"field"`
				}
				var simple []SimplePoint
				for _, p := range points {
					sp := SimplePoint{Week: p.Week, Field: field}
					switch field {
					case "median_price":
						sp.Value = p.AvgPrice
					case "dom":
						sp.Value = p.AvgDOM
					case "inventory":
						sp.Value = float64(p.Inventory)
					}
					simple = append(simple, sp)
				}
				return printJSONFiltered(cmd.OutOrStdout(), simple, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), points, flags)
		},
	}

	cmd.Flags().StringVar(&zip, "zip", "", "Zip code to analyze (required)")
	cmd.Flags().IntVar(&weeks, "weeks", 12, "How many weeks of history to include")
	cmd.Flags().StringVar(&field, "field", "", "Focus on one metric: median_price, dom, or inventory")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func roundTo(v float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(v*factor+0.5)) / factor
}
