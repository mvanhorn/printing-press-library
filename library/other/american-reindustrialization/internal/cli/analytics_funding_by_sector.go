// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type fundingSectorCell struct {
	FundingStage   string `json:"funding_stage"`
	Sector         string `json:"sector"`
	Companies      int64  `json:"companies"`
	TopEmployRange string `json:"top_employee_range,omitempty"`
}

func newAnalyticsFundingBySectorCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "funding-by-sector",
		Short: "Crosstab of funding_stage × primary_sector with company counts and the most common employee_range per cell",
		Long: "GROUP BY funding_stage × primary_sector over locally synced companies. " +
			"Each cell carries the cell's company count plus the most common employee_range " +
			"as a proxy for company scale at that intersection.",
		Example:     "  american-reindustrialization-pp-cli analytics funding-by-sector --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(NULLIF(TRIM(funding_stage), ''), '(unspecified)') AS stage,
				        COALESCE(NULLIF(TRIM(primary_sector), ''), '(unspecified)') AS sector,
				        COALESCE(NULLIF(TRIM(employee_range), ''), '')              AS er
				 FROM companies`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type cellAccum struct {
				count    int64
				erCounts map[string]int
			}
			cells := map[string]*cellAccum{}
			key := func(stage, sector string) string { return stage + "\x1f" + sector }
			for rows.Next() {
				var stage, sector, er sql.NullString
				if err := rows.Scan(&stage, &sector, &er); err != nil {
					continue
				}
				k := key(stage.String, sector.String)
				c := cells[k]
				if c == nil {
					c = &cellAccum{erCounts: map[string]int{}}
					cells[k] = c
				}
				c.count++
				if er.String != "" {
					c.erCounts[er.String]++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating funding-by-sector rows: %w", err)
			}

			out := make([]fundingSectorCell, 0, len(cells))
			for k, c := range cells {
				stage, sector, _ := splitOnce(k, "\x1f")
				out = append(out, fundingSectorCell{
					FundingStage:   stage,
					Sector:         sector,
					Companies:      c.count,
					TopEmployRange: argmaxString(c.erCounts),
				})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Companies != out[j].Companies {
					return out[i].Companies > out[j].Companies
				}
				if out[i].FundingStage != out[j].FundingStage {
					return out[i].FundingStage < out[j].FundingStage
				}
				return out[i].Sector < out[j].Sector
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

func splitOnce(s, sep string) (string, string, bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}

func argmaxString(m map[string]int) string {
	best := ""
	bestN := -1
	for k, v := range m {
		if v > bestN || (v == bestN && k < best) {
			best = k
			bestN = v
		}
	}
	if bestN <= 0 {
		return ""
	}
	return best
}
