// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newChatflowStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var includeDeployed bool

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List chatflows not updated in N days against the local cache",
		Long: `List chatflows whose updated_date is older than --days days against the local
cache. By default, deployed flows are excluded (they may be live in production
and not safe to delete); pass --include-deployed to see those too.

Run ` + "`flowiseai-pp-cli sync`" + ` first to ensure the local cache is fresh.`,
		Example: "  flowiseai-pp-cli chatflow stale --days 60 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days)
			query := `SELECT id, COALESCE(name, '') AS name,
				COALESCE(updated_date, '') AS updated_date,
				COALESCE(deployed, 0) AS deployed,
				COALESCE(category, '') AS category
				FROM chatflows
				WHERE updated_date IS NOT NULL
				  AND updated_date < ?`
			if !includeDeployed {
				query += ` AND (deployed IS NULL OR deployed = 0)`
			}
			query += ` ORDER BY updated_date ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, cutoff.UTC().Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type staleRow struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				UpdatedDate string `json:"updatedDate"`
				DaysStale   int    `json:"daysStale"`
				Deployed    bool   `json:"deployed"`
				Category    string `json:"category"`
			}
			var results []staleRow
			now := time.Now()
			for rows.Next() {
				var r staleRow
				var deployedInt int
				if err := rows.Scan(&r.ID, &r.Name, &r.UpdatedDate, &deployedInt, &r.Category); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				r.Deployed = deployedInt != 0
				if t, parseErr := time.Parse(time.RFC3339, r.UpdatedDate); parseErr == nil {
					r.DaysStale = int(now.Sub(t).Hours() / 24)
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, results)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No chatflows are older than %d days in the local cache. Run `sync` if you haven't recently.\n", days)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "ID\tNAME\tDAYS STALE\tDEPLOYED\tCATEGORY")
			for _, r := range results {
				deployed := "no"
				if r.Deployed {
					deployed = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", r.ID, truncate(r.Name, 40), r.DaysStale, deployed, truncate(strings.TrimSpace(r.Category), 24))
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Show chatflows not updated in this many days")
	cmd.Flags().BoolVar(&includeDeployed, "include-deployed", false, "Include deployed chatflows (default excludes them — they may be live)")
	return cmd
}

// Ensure we don't drop the json import on builds that strip the table path
var _ = json.Marshal
