// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newMarketHeatCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var sortBy string
	var top int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "market-heat",
		Short:       "Rank all synced zip codes from hottest to coldest",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Rank every zip code in the local store by market heat — a composite of
inventory size, median DOM, and average price. Useful for comparing markets
across a metro area or portfolio of zip codes you've synced.

Run 'redfin-pp-cli sync' for each zip to build a multi-zip comparison.`,
		Example: `  # Top 10 hottest zip codes across all synced data
  redfin-pp-cli market-heat --top 10

  # Sort by DOM (fastest-moving markets first)
  redfin-pp-cli market-heat --sort dom --json

  # Only use data from the last 4 weeks
  redfin-pp-cli market-heat --weeks 4 --top 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			validSorts := map[string]bool{"price": true, "dom": true, "inventory": true}
			if sortBy != "" && !validSorts[sortBy] {
				return usageErr(fmt.Errorf("--sort must be one of: price, dom, inventory"))
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
					json_extract(data, '$.address.zip') AS zip,
					COUNT(*) AS inventory,
					AVG(CAST(json_extract(data, '$.dom.value') AS REAL)) AS avg_dom,
					AVG(CAST(json_extract(data, '$.price.value') AS REAL)) AS avg_price
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.status') = 1
				  AND json_extract(data, '$.address.zip') IS NOT NULL
				  AND synced_at >= datetime('now', ?)
				GROUP BY zip
				HAVING COUNT(*) >= 3
				ORDER BY avg_dom ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, weekParam)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync' for one or more zip codes first")
				}
				return fmt.Errorf("querying market heat: %w", err)
			}
			defer rows.Close()

			type ZipHeat struct {
				Zip       string  `json:"zip"`
				Inventory int     `json:"inventory"`
				AvgDOM    float64 `json:"avg_dom"`
				AvgPrice  float64 `json:"avg_price"`
				HeatScore float64 `json:"heat_score"`
				Rank      int     `json:"rank"`
			}

			var results []ZipHeat
			for rows.Next() {
				var z ZipHeat
				var avgDOM, avgPrice *float64
				if err := rows.Scan(&z.Zip, &z.Inventory, &avgDOM, &avgPrice); err != nil {
					continue
				}
				if avgDOM != nil {
					z.AvgDOM = roundTo(*avgDOM, 1)
				}
				if avgPrice != nil {
					z.AvgPrice = roundTo(*avgPrice, 0)
				}
				results = append(results, z)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No data found — run 'redfin-pp-cli sync' for one or more zip codes first.")
				return nil
			}

			// Compute heat score: lower DOM + higher inventory = hotter
			// Normalize each dimension against the range, then combine
			maxDOM, minDOM := results[0].AvgDOM, results[0].AvgDOM
			maxInv, minInv := float64(results[0].Inventory), float64(results[0].Inventory)
			for _, r := range results {
				if r.AvgDOM > maxDOM {
					maxDOM = r.AvgDOM
				}
				if r.AvgDOM < minDOM {
					minDOM = r.AvgDOM
				}
				if float64(r.Inventory) > maxInv {
					maxInv = float64(r.Inventory)
				}
				if float64(r.Inventory) < minInv {
					minInv = float64(r.Inventory)
				}
			}

			for i := range results {
				r := &results[i]
				domNorm := 0.5
				if maxDOM != minDOM {
					domNorm = 1 - (r.AvgDOM-minDOM)/(maxDOM-minDOM) // lower DOM = hotter
				}
				invNorm := 0.5
				if maxInv != minInv {
					invNorm = (float64(r.Inventory) - minInv) / (maxInv - minInv)
				}
				r.HeatScore = roundTo((domNorm*0.7+invNorm*0.3)*10, 1)
			}

			// Sort
			switch sortBy {
			case "dom":
				sort.Slice(results, func(i, j int) bool { return results[i].AvgDOM < results[j].AvgDOM })
			case "inventory":
				sort.Slice(results, func(i, j int) bool { return results[i].Inventory > results[j].Inventory })
			case "price":
				sort.Slice(results, func(i, j int) bool { return results[i].AvgPrice > results[j].AvgPrice })
			default:
				sort.Slice(results, func(i, j int) bool { return results[i].HeatScore > results[j].HeatScore })
			}

			for i := range results {
				results[i].Rank = i + 1
			}

			if top > 0 && len(results) > top {
				results = results[:top]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().IntVar(&weeks, "weeks", 8, "How many weeks of history to include")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort by: price, dom, or inventory (default: heat_score)")
	cmd.Flags().IntVar(&top, "top", 0, "Only show top N zip codes (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
