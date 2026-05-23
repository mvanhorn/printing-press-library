// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find stale deployments not updated in N days",
		Long: `Find deployments across all projects that haven't been updated in N days.

Useful for identifying deployments that may need attention, cleanup, or cost
optimization. Requires synced data - run 'sync' first to populate local cache.`,
		Example: `  vibecode-pp-cli stale --days 14
  vibecode-pp-cli stale --days 30 --json
  vibecode-pp-cli stale --days 7 --select id,name,status`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for stale deployments")
				return nil
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days)

			// Query for deployments not updated since cutoff
			// The resources table stores deployment data with resource_type='deployments'
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data, updated_at FROM resources
				WHERE resource_type IN ('deployments', 'projects_deployments')
				  AND updated_at < ?
				ORDER BY updated_at ASC
			`, cutoff.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying stale deployments: %w", err)
			}
			defer rows.Close()

			type staleDeployment struct {
				ID              string    `json:"id"`
				Data            any       `json:"data,omitempty"`
				UpdatedAt       time.Time `json:"updated_at"`
				DaysSinceUpdate int       `json:"days_since_update"`
			}

			var results []staleDeployment
			for rows.Next() {
				var id, dataStr string
				var updatedAt time.Time
				if err := rows.Scan(&id, &dataStr, &updatedAt); err != nil {
					continue
				}
				daysSince := int(time.Since(updatedAt).Hours() / 24)
				results = append(results, staleDeployment{
					ID:              id,
					UpdatedAt:       updatedAt,
					DaysSinceUpdate: daysSince,
				})
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale deployments found (older than %d days)\n", days)
				return nil
			}

			// Table output
			headers := []string{"ID", "Updated At", "Days Stale"}
			var tableRows [][]string
			for _, r := range results {
				tableRows = append(tableRows, []string{
					r.ID,
					r.UpdatedAt.Format("2006-01-02"),
					fmt.Sprintf("%d", r.DaysSinceUpdate),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Number of days threshold")
	return cmd
}
