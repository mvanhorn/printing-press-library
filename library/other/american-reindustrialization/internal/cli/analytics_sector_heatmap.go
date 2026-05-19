// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type heatmapCell struct {
	Sector string `json:"sector"`
	State  string `json:"state"`
	Value  int64  `json:"value"`
}

func newAnalyticsSectorHeatmapCmd(flags *rootFlags) *cobra.Command {
	var fundingStage, weight, dbPath string

	cmd := &cobra.Command{
		Use:   "sector-heatmap",
		Short: "Crosstab of primary_sector × HQ state with company counts (or jobs_count weights)",
		Long: "GROUP BY primary_sector × hq_state over locally synced companies. " +
			"Use --weight jobs to sum jobs_count per cell instead of counting companies. " +
			"Use --funding-stage to filter rows before aggregating.",
		Example:     "  american-reindustrialization-pp-cli analytics sector-heatmap --funding-stage seed --weight jobs --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			weight = strings.TrimSpace(strings.ToLower(weight))
			if weight != "" && weight != "companies" && weight != "jobs" {
				return usageErr(fmt.Errorf("invalid --weight %q: use 'companies' (default) or 'jobs'", weight))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			aggExpr := "COUNT(*)"
			if weight == "jobs" {
				aggExpr = "COALESCE(SUM(jobs_count), 0)"
			}
			q := fmt.Sprintf(`SELECT COALESCE(NULLIF(TRIM(primary_sector), ''), '(unspecified)') AS sector,
			                          COALESCE(NULLIF(TRIM(hq_state), ''), '(unspecified)') AS state,
			                          %s AS value
			                   FROM companies
			                   WHERE 1=1`, aggExpr)
			args2 := []any{}
			if fundingStage != "" {
				q += " AND lower(funding_stage) = lower(?)"
				args2 = append(args2, strings.TrimSpace(fundingStage))
			}
			q += " GROUP BY sector, state ORDER BY value DESC, sector ASC, state ASC"

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := make([]heatmapCell, 0)
			for rows.Next() {
				var sector, state sql.NullString
				var value sql.NullInt64
				if err := rows.Scan(&sector, &state, &value); err != nil {
					continue
				}
				if value.Int64 == 0 {
					continue
				}
				out = append(out, heatmapCell{
					Sector: sector.String,
					State:  state.String,
					Value:  value.Int64,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating sector-heatmap rows: %w", err)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Value != out[j].Value {
					return out[i].Value > out[j].Value
				}
				if out[i].Sector != out[j].Sector {
					return out[i].Sector < out[j].Sector
				}
				return out[i].State < out[j].State
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&fundingStage, "funding-stage", "", "Restrict to a single funding_stage")
	cmd.Flags().StringVar(&weight, "weight", "companies", "Aggregation weight: companies (default) or jobs")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}
