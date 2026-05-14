// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// sectorBreadthResult holds breadth metrics for a named index.
type sectorBreadthResult struct {
	Index         string  `json:"index"`
	Constituents  int     `json:"constituents"`
	Advances      int     `json:"advances"`
	Declines      int     `json:"declines"`
	Unchanged     int     `json:"unchanged"`
	ADRatio       float64 `json:"ad_ratio"`
	MedianPChange float64 `json:"median_p_change"`
	DeliveryAvg   float64 `json:"avg_delivery_pct"`
	Verdict       string  `json:"verdict"`
}

func newSectorBreadthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sector string
	var limit int

	cmd := &cobra.Command{
		Use:   "sector-breadth",
		Short: "Advance/decline ratio and delivery breadth for index constituents — richer than the headline index number",
		Long: `Computes advance/decline ratio, median pChange, and average delivery%
for every constituent of a named index by joining index constituents
with equity quote data in the local store.

This cross-table join is impossible from any single NSE API call and
only becomes available after syncing both index constituents and equity
quote data via 'nse-india-pp-cli equity quote --symbol <SYMBOL>'.`,
		Example: `  # Breadth for all available indices
  nse-india-pp-cli sector-breadth

  # Breadth for NIFTY IT sector
  nse-india-pp-cli sector-breadth --sector IT

  # Agent-friendly output
  nse-india-pp-cli sector-breadth --sector "NIFTY 50" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] sector-breadth: would join index constituents x equity quotes in local store`)
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nse-india-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			// Fetch all index constituent rows from the store.
			// The indices constituents endpoint returns an array of stock objects
			// each with a symbol, pChange, and meta.indexName.
			whereClause := ""
			var queryArgs []any
			if sector != "" {
				whereClause = "AND (UPPER(json_extract(data, '$.meta.indexName')) LIKE UPPER('%' || ? || '%') OR UPPER(json_extract(data, '$.index')) LIKE UPPER('%' || ? || '%'))"
				queryArgs = append(queryArgs, sector, sector)
			}

			query := fmt.Sprintf(`
				SELECT json_extract(data, '$.meta.indexName') as index_name,
				       json_extract(data, '$.symbol') as symbol,
				       json_extract(data, '$.pChange') as p_change,
				       json_extract(data, '$.deliveryToTradedQty') as delivery_pct
				FROM resources
				WHERE resource_type IN ('indices', 'index_constituents', 'equity-stockIndices')
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND json_extract(data, '$.pChange') IS NOT NULL
				  %s
				ORDER BY index_name, symbol
			`, whereClause)

			rows, err := s.DB().Query(query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying store: %w\nhint: run 'nse-india-pp-cli equity quote --symbol <SYMBOL>' for each symbol to populate the local store", err)
			}
			defer rows.Close()

			type constituent struct {
				symbol   string
				pChange  float64
				delivPct float64
			}
			byIndex := map[string][]constituent{}
			for rows.Next() {
				var indexName, symbol string
				var pChangeRaw, delivRaw *string
				if err := rows.Scan(&indexName, &symbol, &pChangeRaw, &delivRaw); err != nil {
					continue
				}
				if indexName == "" || symbol == "" {
					continue
				}
				pc := 0.0
				if pChangeRaw != nil {
					pc, _ = strconv.ParseFloat(*pChangeRaw, 64)
				}
				deliv := 0.0
				if delivRaw != nil {
					deliv, _ = strconv.ParseFloat(*delivRaw, 64)
				}
				byIndex[indexName] = append(byIndex[indexName], constituent{symbol: symbol, pChange: pc, delivPct: deliv})
			}

			if len(byIndex) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No index constituent data found. Run 'nse-india-pp-cli indices constituents --index \"NIFTY 50\"' to populate.")
				return nil
			}

			var results []sectorBreadthResult
			for indexName, constituents := range byIndex {
				advances, declines, unchanged := 0, 0, 0
				var pChanges []float64
				delivSum := 0.0
				delivCount := 0
				for _, c := range constituents {
					if c.pChange > 0 {
						advances++
					} else if c.pChange < 0 {
						declines++
					} else {
						unchanged++
					}
					pChanges = append(pChanges, c.pChange)
					if c.delivPct > 0 {
						delivSum += c.delivPct
						delivCount++
					}
				}
				sort.Float64s(pChanges)
				median := 0.0
				n := len(pChanges)
				if n > 0 {
					if n%2 == 0 {
						median = (pChanges[n/2-1] + pChanges[n/2]) / 2
					} else {
						median = pChanges[n/2]
					}
				}
				adRatio := 0.0
				if declines > 0 {
					adRatio = float64(advances) / float64(declines)
				} else if advances > 0 {
					adRatio = float64(advances) // all advances, no declines
				}
				delivAvg := 0.0
				if delivCount > 0 {
					delivAvg = delivSum / float64(delivCount)
				}
				verdict := "bearish"
				if adRatio >= 2.0 {
					verdict = "strongly-bullish"
				} else if adRatio >= 1.2 {
					verdict = "bullish"
				} else if adRatio >= 0.8 {
					verdict = "neutral"
				} else if adRatio >= 0.5 {
					verdict = "mildly-bearish"
				}

				results = append(results, sectorBreadthResult{
					Index:         indexName,
					Constituents:  len(constituents),
					Advances:      advances,
					Declines:      declines,
					Unchanged:     unchanged,
					ADRatio:       adRatio,
					MedianPChange: median,
					DeliveryAvg:   delivAvg,
					Verdict:       verdict,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].ADRatio > results[j].ADRatio
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Sector Breadth Analysis\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s %5s %4s %4s %4s %7s %9s %9s %s\n",
				"INDEX", "N", "ADV", "DEC", "UNC", "A/D", "MED_PC%", "DELIV%", "VERDICT")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				name := r.Index
				if len(name) > 26 {
					name = name[:23] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %5d %4d %4d %4d %7.2f %9.2f %9.1f %s\n",
					name, r.Constituents, r.Advances, r.Declines, r.Unchanged,
					r.ADRatio, r.MedianPChange, r.DeliveryAvg, r.Verdict)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().StringVar(&sector, "sector", "", "Filter by sector/index name (e.g. IT, BANK, 'NIFTY 50')")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")

	return cmd
}
