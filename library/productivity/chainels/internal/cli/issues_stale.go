package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type staleIssueRow struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Company      string `json:"company"`
	LastActivity string `json:"last_activity"`
	DaysIdle     int    `json:"days_idle"`
}

func newIssuesStaleCmd(flags *rootFlags) *cobra.Command {
	var olderThan string
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:     "stale",
		Short:   "List issues with no state change for N days, cross-community",
		Long:    "Reads from the local store synced by `chainels-pp-cli sync`. Open issues whose last_activity is older than --older-than are listed across every community in the store.",
		Example: "  chainels-pp-cli issues stale --older-than 14d --json",
		Annotations: map[string]string{
			"mcp:read-only":        "true",
			"pp:typed-exit-codes":  "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			days, err := parseDaysFlag(olderThan)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(title,''), COALESCE(status,''), COALESCE(company,''),
				       COALESCE(last_activity, created_at, '')
				FROM issues
				WHERE COALESCE(last_activity, created_at) < ?
				  AND (status IS NULL OR LOWER(status) NOT IN ('closed','resolved','done'))
				ORDER BY COALESCE(last_activity, created_at) ASC
				LIMIT ?`, cutoff.Format(time.RFC3339), limit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			out := make([]staleIssueRow, 0)
			for rows.Next() {
				var i staleIssueRow
				var ts sql.NullString
				if err := rows.Scan(&i.ID, &i.Title, &i.Status, &i.Company, &ts); err != nil {
					return err
				}
				if ts.Valid {
					i.LastActivity = ts.String
					if t, err := time.Parse(time.RFC3339, ts.String); err == nil {
						i.DaysIdle = int(time.Since(t).Hours() / 24)
					}
				}
				out = append(out, i)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "14d", "Mark issues stale older than N days (14d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum rows to return")
	return cmd
}
