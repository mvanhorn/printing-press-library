// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newSellerPulseCmd(flags *rootFlags) *cobra.Command {
	var zip string
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "seller-pulse",
		Short:       "Get a seller-oriented market snapshot for a zip code",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compute seller-side metrics: total inventory, median DOM, average list price,
and the percentage of active listings that are stale (60+ DOM). Useful for
understanding how hard it is to sell in a given zip right now.

Run 'redfin-pp-cli sync' first to populate local data.`,
		Example: `  # Seller snapshot for zip 94102
  redfin-pp-cli seller-pulse --zip 94102

  # Last 4 weeks of data only
  redfin-pp-cli seller-pulse --zip 94102 --weeks 4 --json`,
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

			weekParam := fmt.Sprintf("-%d weeks", weeks)
			query := `
				SELECT
					COUNT(*) AS total_active,
					AVG(CAST(json_extract(data, '$.dom.value') AS REAL)) AS avg_dom,
					AVG(CAST(json_extract(data, '$.price.value') AS REAL)) AS avg_price,
					SUM(CASE WHEN CAST(json_extract(data, '$.dom.value') AS INTEGER) >= 60 THEN 1 ELSE 0 END) AS stale_count,
					MIN(CAST(json_extract(data, '$.price.value') AS REAL)) AS min_price,
					MAX(CAST(json_extract(data, '$.price.value') AS REAL)) AS max_price
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.address.zip') = ?
				  AND json_extract(data, '$.status') = 1
				  AND synced_at >= datetime('now', ?)`

			rows, err := db.DB().QueryContext(cmd.Context(), query, zip, weekParam)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", zip)
				}
				return fmt.Errorf("querying seller pulse: %w", err)
			}
			defer rows.Close()

			if !rows.Next() {
				fmt.Fprintf(cmd.OutOrStdout(), "No active listings found for zip %s.\n", zip)
				return nil
			}

			var totalActive, staleCount int
			var avgDOM, avgPrice, minPrice, maxPrice *float64
			if err := rows.Scan(&totalActive, &avgDOM, &avgPrice, &staleCount, &minPrice, &maxPrice); err != nil {
				return fmt.Errorf("scanning result: %w", err)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			stalePct := 0.0
			if totalActive > 0 {
				stalePct = roundTo(float64(staleCount)/float64(totalActive)*100, 1)
			}

			result := map[string]any{
				"zip":                     zip,
				"weeks_window":            weeks,
				"total_active_listings":   totalActive,
				"stale_listings_60d_plus": staleCount,
				"stale_pct":               stalePct,
			}
			if avgDOM != nil {
				result["avg_dom"] = roundTo(*avgDOM, 1)
			}
			if avgPrice != nil {
				result["avg_list_price"] = roundTo(*avgPrice, 0)
			}
			if minPrice != nil {
				result["min_list_price"] = roundTo(*minPrice, 0)
			}
			if maxPrice != nil {
				result["max_list_price"] = roundTo(*maxPrice, 0)
			}

			// Seller condition interpretation
			interpretation := "neutral"
			if avgDOM != nil {
				switch {
				case *avgDOM < 14:
					interpretation = "hot (sellers market — homes moving fast)"
				case *avgDOM < 30:
					interpretation = "warm (balanced with seller edge)"
				case *avgDOM < 60:
					interpretation = "cool (balanced with buyer edge)"
				default:
					interpretation = "cold (buyers market — high inventory, slow sales)"
				}
			}
			result["market_condition"] = interpretation

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&zip, "zip", "", "Zip code to analyze (required)")
	cmd.Flags().IntVar(&weeks, "weeks", 8, "How many weeks of history to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
