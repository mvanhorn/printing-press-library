package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newTPDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor-tp",
		Short: "Check Trustpilot-specific local cache health",
		Long:  "Checks the Trustpilot-specific SQLite tables used by sync-trustpilot, search-reviews, and local read commands.",
		Example: `  trustpilot-pp-cli doctor-tp
  trustpilot-pp-cli doctor-tp --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := collectTPDoctorReport(cmd.Context())
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Trustpilot cache: %s\n", report["summary"])
			fmt.Fprintf(cmd.OutOrStdout(), "  db_path: %s\n", report["dbPath"])
			if harvestedAt, _ := report["sessionHarvestedAt"].(string); harvestedAt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  session_harvested_at: %s\n", harvestedAt)
			}
			return nil
		},
	}
}

// PATCH: Add Trustpilot-specific doctor output without editing generated doctor.go.
func collectTPDoctorReport(ctx context.Context) (map[string]any, error) {
	db, err := openTPStore(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	reviews, err := countTPRows(ctx, db, "tp_reviews")
	if err != nil {
		return nil, err
	}
	companies, err := countTPRows(ctx, db, "tp_companies")
	if err != nil {
		return nil, err
	}
	cursors, err := countTPRows(ctx, db, "tp_sync_cursors")
	if err != nil {
		return nil, err
	}
	var token sql.NullString
	var harvestedAt sql.NullString
	_ = db.QueryRowContext(ctx, `SELECT aws_waf_token, harvested_at FROM tp_session WHERE id = 1`).Scan(&token, &harvestedAt)

	status := "ok"
	summary := fmt.Sprintf("ok (%d reviews, %d companies, %d cursors)", reviews, companies, cursors)
	if reviews == 0 {
		status = "empty"
		summary = "empty (0 reviews; run sync-trustpilot)"
		if token.Valid && token.String != "" && harvestedAt.Valid && harvestedAt.String != "" {
			summary = fmt.Sprintf("empty (0 reviews; session ok, harvested %s; run sync-trustpilot)", harvestedAt.String)
		}
	}

	return map[string]any{
		"status":             status,
		"summary":            summary,
		"dbPath":             tpDBPath(),
		"reviews":            reviews,
		"companies":          companies,
		"cursors":            cursors,
		"hasSession":         token.Valid && token.String != "",
		"sessionHarvestedAt": harvestedAt.String,
	}, nil
}

func countTPRows(ctx context.Context, db *sql.DB, table string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
