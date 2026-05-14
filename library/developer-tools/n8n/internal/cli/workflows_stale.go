// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/store"
)

type staleWorkflow struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Active          bool      `json:"active"`
	UpdatedAt       time.Time `json:"updated_at"`
	DaysSinceUpdate int       `json:"days_since_update"`
	IsArchived      bool      `json:"is_archived,omitempty"`
}

func newWorkflowsStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var onlyActive bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List workflows not updated within a time window",
		Long: `List workflows that have not been updated in the given number of days.
Useful for identifying dormant automations that should be reviewed, archived,
or cleaned up. Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Workflows not updated in 90 days
  n8n-pp-cli workflows stale --days 90

  # Only active (running) workflows that may have drifted
  n8n-pp-cli workflows stale --days 30 --active

  # JSON output for scripting
  n8n-pp-cli workflows stale --days 60 --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("n8n-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().AddDate(0, 0, -days)
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, name, active, updated_at, is_archived
				   FROM workflows
				  WHERE updated_at < ?
				  ORDER BY updated_at ASC`,
				cutoff.Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("querying workflows: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer rows.Close()

			var results []staleWorkflow
			now := time.Now().UTC()
			for rows.Next() {
				var w staleWorkflow
				var updatedStr string
				var archivedInt int
				if err := rows.Scan(&w.ID, &w.Name, &w.Active, &updatedStr, &archivedInt); err != nil {
					continue
				}
				w.IsArchived = archivedInt != 0
				if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
					w.UpdatedAt = t
					w.DaysSinceUpdate = int(now.Sub(t).Hours() / 24)
				}
				if onlyActive && !w.Active {
					continue
				}
				results = append(results, w)
			}
			if rows.Err() != nil {
				return fmt.Errorf("scanning rows: %w", rows.Err())
			}

			if len(results) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No stale workflows (threshold: %d days)\n", days)
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Flag workflows not updated in this many days")
	cmd.Flags().BoolVar(&onlyActive, "active", false, "Only show active (enabled) workflows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: ~/.config/n8n-pp-cli/store.db)")
	return cmd
}
