// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// indexDriverResult holds one stock's point contribution to an index move.
type indexDriverResult struct {
	Symbol       string  `json:"symbol"`
	Company      string  `json:"company_name"`
	Weight       float64 `json:"weight_pct"`
	PChange      float64 `json:"p_change_pct"`
	Contribution float64 `json:"index_point_contribution"`
	CumPct       float64 `json:"cumulative_pct_of_move"`
	Impact       string  `json:"impact"`
}

func newIndexDriverCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var indexName string
	var topN int

	cmd := &cobra.Command{
		Use:   "index-driver",
		Short: "Decompose an index move into per-stock point contributions — find the 3-5 stocks driving 80% of the change",
		Long: `Joins index constituent weights with daily pChange to compute each stock's
weighted contribution to the index move. Identifies whether a rally is
broad-based or concentrated in a handful of names.

Requires: index constituents populated via 'nse-india-pp-cli indices constituents
--index "<INDEX>"' and stored in the local database.`,
		Example: `  # Decompose today's NIFTY 50 move
  nse-india-pp-cli index-driver --index "NIFTY 50"

  # Show top 10 contributors to NIFTY BANK
  nse-india-pp-cli index-driver --index "NIFTY BANK" --top 10

  # Agent output
  nse-india-pp-cli index-driver --index "NIFTY 50" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] index-driver: would join index weights x pChange in local store`)
				return nil
			}
			if indexName == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--index is required (e.g. 'NIFTY 50', 'NIFTY BANK')"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nse-india-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			// Fetch all constituents from the index, with their weightage and pChange.
			// The equity-stockIndices endpoint returns weightage and pChange per constituent.
			rows, err := s.DB().Query(`
				SELECT json_extract(data, '$.symbol') as symbol,
				       COALESCE(json_extract(data, '$.meta.companyName'), json_extract(data, '$.symbol')) as company,
				       json_extract(data, '$.weightage') as weightage,
				       json_extract(data, '$.pChange') as p_change
				FROM resources
				WHERE resource_type IN ('indices', 'index_constituents', 'equity-stockIndices')
				  AND json_extract(data, '$.symbol') IS NOT NULL
				  AND json_extract(data, '$.pChange') IS NOT NULL
				  AND (
				        UPPER(json_extract(data, '$.meta.indexName')) LIKE UPPER('%' || ? || '%')
				     OR UPPER(json_extract(data, '$.index')) LIKE UPPER('%' || ? || '%')
				     OR ? = ''
				  )
				ORDER BY CAST(COALESCE(json_extract(data, '$.weightage'), '0') AS REAL) DESC
			`, indexName, indexName, indexName)
			if err != nil {
				return fmt.Errorf("querying store: %w\nhint: run 'nse-india-pp-cli indices constituents --index \"%s\"' first", err, indexName)
			}
			defer rows.Close()

			type constituent struct {
				symbol  string
				company string
				weight  float64
				pChange float64
			}
			var constituents []constituent
			for rows.Next() {
				var symbol, company string
				var weightRaw, pChangeRaw *string
				if err := rows.Scan(&symbol, &company, &weightRaw, &pChangeRaw); err != nil {
					continue
				}
				if symbol == "" {
					continue
				}
				w := 0.0
				if weightRaw != nil {
					w, _ = strconv.ParseFloat(*weightRaw, 64)
				}
				pc := 0.0
				if pChangeRaw != nil {
					pc, _ = strconv.ParseFloat(*pChangeRaw, 64)
				}
				constituents = append(constituents, constituent{symbol: symbol, company: company, weight: w, pChange: pc})
			}

			if len(constituents) == 0 {
				return fmt.Errorf("no constituent data found for %q\nhint: run 'nse-india-pp-cli indices constituents --index \"%s\"'", indexName, indexName)
			}

			// Compute point contributions (weight * pChange / 100 approximation)
			totalContrib := 0.0
			var results []indexDriverResult
			for _, c := range constituents {
				contrib := c.weight * c.pChange / 100
				totalContrib += contrib
				impact := "neutral"
				if contrib > 0.1 {
					impact = "positive"
				} else if contrib < -0.1 {
					impact = "negative"
				}
				results = append(results, indexDriverResult{
					Symbol:       c.symbol,
					Company:      c.company,
					Weight:       c.weight,
					PChange:      c.pChange,
					Contribution: contrib,
					Impact:       impact,
				})
			}

			// Sort by absolute contribution descending
			sort.Slice(results, func(i, j int) bool {
				return math.Abs(results[i].Contribution) > math.Abs(results[j].Contribution)
			})

			// Compute cumulative % of total move
			cumAbs := 0.0
			totalAbs := 0.0
			for _, r := range results {
				totalAbs += math.Abs(r.Contribution)
			}
			for i := range results {
				cumAbs += math.Abs(results[i].Contribution)
				if totalAbs > 0 {
					results[i].CumPct = cumAbs / totalAbs * 100
				}
			}

			if topN > 0 && len(results) > topN {
				results = results[:topN]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			direction := "up"
			if totalContrib < 0 {
				direction = "down"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Index Driver: %s (total move: %+.2f points %s)\n\n", indexName, totalContrib, direction)
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-24s %7s %8s %10s %8s %s\n",
				"SYMBOL", "COMPANY", "WEIGHT%", "P_CHG%", "CONTRIB", "CUM%", "IMPACT")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				company := r.Company
				if len(company) > 22 {
					company = company[:19] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-24s %7.2f %8.2f %10.3f %8.1f %s\n",
					r.Symbol, company, r.Weight, r.PChange, r.Contribution, r.CumPct, r.Impact)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().StringVar(&indexName, "index", "", "Index name (e.g. 'NIFTY 50', 'NIFTY BANK')")
	cmd.Flags().IntVar(&topN, "top", 15, "Show top N contributors by absolute contribution")

	return cmd
}
