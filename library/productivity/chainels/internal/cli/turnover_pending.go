package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type turnoverPendingRow struct {
	CommunityID string `json:"community_id"`
	EntityID    string `json:"entity_id"`
	Period      string `json:"period"`
	Reason      string `json:"reason"`
}

func newTurnoverPendingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var period string
	cmd := &cobra.Command{
		Use:     "pending",
		Short:   "List tenants who haven't submitted a turnover report for a period",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Set-difference between expected submitters (companies tagged with turnover schemes) and actual reports for --period (YYYY-MM). Requires `read.turnover` scope on the OAuth app.",
		Example: "  chainels-pp-cli turnover pending --period 2026-04 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if period == "" {
				return fmt.Errorf("--period is required (YYYY-MM, e.g. 2026-04)")
			}
			if len(period) != 7 || period[4] != '-' {
				return fmt.Errorf("--period must be YYYY-MM, got %q", period)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			// Expected submitters: companies with a turnover row (the companies_turnover
			// table holds the per-company turnover-scheme membership). Subtract any
			// company whose synced report data references the requested period.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT DISTINCT ct.companies_id AS community_id,
				                COALESCE(json_extract(ct.data,'$.entity_id'), '') AS entity_id
				FROM companies_turnover ct
				WHERE NOT EXISTS (
					SELECT 1 FROM companies_turnover ct2
					WHERE ct2.companies_id = ct.companies_id
					  AND ct2.data LIKE '%' || ? || '%'
				)
				ORDER BY community_id, entity_id`, period)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			out := make([]turnoverPendingRow, 0)
			for rows.Next() {
				var r turnoverPendingRow
				var entity sql.NullString
				if err := rows.Scan(&r.CommunityID, &entity); err != nil {
					return err
				}
				if entity.Valid {
					r.EntityID = entity.String
				}
				r.Period = period
				r.Reason = "no report row mentions period"
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().StringVar(&period, "period", "", "Reporting period in YYYY-MM form (required)")
	return cmd
}
