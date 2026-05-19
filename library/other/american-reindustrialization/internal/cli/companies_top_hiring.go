// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type topHiringRow struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	JobsCount    int64  `json:"jobs_count"`
	Sector       string `json:"primary_sector,omitempty"`
	State        string `json:"hq_state,omitempty"`
	FundingStage string `json:"funding_stage,omitempty"`
}

func newCompaniesTopHiringCmd(flags *rootFlags) *cobra.Command {
	var sector, state, fundingStage, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "top-hiring",
		Short: "Rank companies by jobs_count descending, with optional filters",
		Long: "Rank locally synced companies by their reported jobs_count, optionally " +
			"filtered by primary_sector, hq_state, or funding_stage. The site has no " +
			"ranking view; this is pure local SQL.",
		Example:     "  american-reindustrialization-pp-cli companies top-hiring --sector robotics --limit 10 --json",
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

			q := `SELECT slug, name, COALESCE(jobs_count, 0) AS jobs_count,
			             COALESCE(primary_sector, ''), COALESCE(hq_state, ''), COALESCE(funding_stage, '')
			      FROM companies
			      WHERE 1=1`
			args2 := []any{}
			if sector != "" {
				q += " AND lower(primary_sector) = lower(?)"
				args2 = append(args2, strings.TrimSpace(sector))
			}
			if state != "" {
				q += " AND upper(hq_state) = upper(?)"
				args2 = append(args2, strings.TrimSpace(state))
			}
			if fundingStage != "" {
				q += " AND lower(funding_stage) = lower(?)"
				args2 = append(args2, strings.TrimSpace(fundingStage))
			}
			q += " ORDER BY jobs_count DESC, name ASC"
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := make([]topHiringRow, 0)
			for rows.Next() {
				var slug, name, sector, st, funding sql.NullString
				var count sql.NullInt64
				if err := rows.Scan(&slug, &name, &count, &sector, &st, &funding); err != nil {
					continue
				}
				out = append(out, topHiringRow{
					Slug:         slug.String,
					Name:         name.String,
					JobsCount:    count.Int64,
					Sector:       sector.String,
					State:        st.String,
					FundingStage: funding.String,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating top-hiring rows: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&sector, "sector", "", "Filter by primary_sector (e.g. robotics)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by HQ state (e.g. TX)")
	cmd.Flags().StringVar(&fundingStage, "funding-stage", "", "Filter by funding_stage (e.g. seed, series-a)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}
