package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type issueLoadRow struct {
	Assignee     string `json:"assignee"`
	Total        int    `json:"total"`
	NewLt7       int    `json:"new_lt_7d"`
	Mid7To30     int    `json:"mid_7_30d"`
	StaleGt30    int    `json:"stale_gt_30d"`
}

func newIssuesLoadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var community string
	cmd := &cobra.Command{
		Use:     "load",
		Short:   "Group open issues by assignee with age buckets",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Open issues are grouped by assignee (via the issues_assignees junction table) and bucketed by last_activity age. Closed/resolved issues are excluded.",
		Example: "  chainels-pp-cli issues load --community <community-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			communityFilter := ""
			args2 := []any{}
			if community != "" {
				communityFilter = "AND COALESCE(company,'') = ?"
				args2 = append(args2, community)
			}
			q := `
				SELECT
					COALESCE(NULLIF(account,''), '<unassigned>') AS assignee,
					COUNT(*) AS total,
					SUM(CASE WHEN julianday('now') - julianday(COALESCE(last_activity, created_at)) < 7 THEN 1 ELSE 0 END) AS new_lt_7d,
					SUM(CASE WHEN julianday('now') - julianday(COALESCE(last_activity, created_at)) BETWEEN 7 AND 30 THEN 1 ELSE 0 END) AS mid_7_30d,
					SUM(CASE WHEN julianday('now') - julianday(COALESCE(last_activity, created_at)) > 30 THEN 1 ELSE 0 END) AS stale_gt_30d
				FROM issues
				WHERE (status IS NULL OR LOWER(status) NOT IN ('closed','resolved','done'))
				` + communityFilter + `
				GROUP BY assignee
				ORDER BY total DESC, assignee ASC`
			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			out := make([]issueLoadRow, 0)
			for rows.Next() {
				var r issueLoadRow
				var nLt7, nMid, nStale sql.NullInt64
				if err := rows.Scan(&r.Assignee, &r.Total, &nLt7, &nMid, &nStale); err != nil {
					return err
				}
				r.NewLt7 = int(nLt7.Int64)
				r.Mid7To30 = int(nMid.Int64)
				r.StaleGt30 = int(nStale.Int64)
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().StringVar(&community, "community", "", "Filter to a single community (company id)")
	return cmd
}
