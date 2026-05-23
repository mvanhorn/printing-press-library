// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var sinceLastSync bool

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Show what changed since a timestamp",
		Long: `Show what changed across all projects since a timestamp or last sync.

Perfect for AI agents resuming work after context switches, or developers
catching up after time away. Requires synced data - run 'sync' first.`,
		Example: `  vibecode-pp-cli changes --since "2 hours ago"
  vibecode-pp-cli changes --since "2024-01-01"
  vibecode-pp-cli changes --since-last-sync
  vibecode-pp-cli changes --since "1 day ago" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for changes")
				return nil
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var cutoff time.Time
			if sinceLastSync {
				// Get the last sync time from metadata
				var lastSyncStr string
				err := db.DB().QueryRowContext(cmd.Context(),
					`SELECT value FROM metadata WHERE key = 'last_sync_at'`).Scan(&lastSyncStr)
				if err != nil {
					return fmt.Errorf("no previous sync found; run 'sync' first")
				}
				cutoff, err = time.Parse(time.RFC3339, lastSyncStr)
				if err != nil {
					return fmt.Errorf("invalid last sync timestamp: %w", err)
				}
			} else if since != "" {
				cutoff, err = parseTimeExpression(since)
				if err != nil {
					return fmt.Errorf("invalid --since value: %w", err)
				}
			} else {
				return fmt.Errorf("specify --since or --since-last-sync")
			}

			// Query for all resources updated since cutoff
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT resource_type, id, updated_at FROM resources
				WHERE updated_at >= ?
				ORDER BY updated_at DESC
			`, cutoff.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying changes: %w", err)
			}
			defer rows.Close()

			type change struct {
				ResourceType string    `json:"resource_type"`
				ID           string    `json:"id"`
				UpdatedAt    time.Time `json:"updated_at"`
			}

			var results []change
			for rows.Next() {
				var resourceType, id string
				var updatedAt time.Time
				if err := rows.Scan(&resourceType, &id, &updatedAt); err != nil {
					continue
				}
				results = append(results, change{
					ResourceType: resourceType,
					ID:           id,
					UpdatedAt:    updatedAt,
				})
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No changes since %s\n", cutoff.Format("2006-01-02 15:04"))
				return nil
			}

			// Table output
			headers := []string{"Type", "ID", "Updated At"}
			var tableRows [][]string
			for _, r := range results {
				tableRows = append(tableRows, []string{
					r.ResourceType,
					r.ID,
					r.UpdatedAt.Format("2006-01-02 15:04"),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Changes since %s:\n\n", cutoff.Format("2006-01-02 15:04"))
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Time expression (e.g. '2 hours ago', '2024-01-01')")
	cmd.Flags().BoolVar(&sinceLastSync, "since-last-sync", false, "Show changes since last sync")
	return cmd
}

// parseTimeExpression parses human-readable time expressions like "2 hours ago"
func parseTimeExpression(expr string) (time.Time, error) {
	expr = strings.TrimSpace(strings.ToLower(expr))

	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, expr); err == nil {
		return t, nil
	}

	// Try date-only format
	if t, err := time.Parse("2006-01-02", expr); err == nil {
		return t, nil
	}

	// Parse relative expressions like "2 hours ago", "1 day ago"
	if strings.HasSuffix(expr, " ago") {
		parts := strings.Fields(strings.TrimSuffix(expr, " ago"))
		if len(parts) == 2 {
			var num int
			if _, err := fmt.Sscanf(parts[0], "%d", &num); err == nil {
				unit := parts[1]
				switch {
				case strings.HasPrefix(unit, "hour"):
					return time.Now().Add(-time.Duration(num) * time.Hour), nil
				case strings.HasPrefix(unit, "day"):
					return time.Now().AddDate(0, 0, -num), nil
				case strings.HasPrefix(unit, "week"):
					return time.Now().AddDate(0, 0, -num*7), nil
				case strings.HasPrefix(unit, "month"):
					return time.Now().AddDate(0, -num, 0), nil
				case strings.HasPrefix(unit, "minute"):
					return time.Now().Add(-time.Duration(num) * time.Minute), nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse time expression: %s", expr)
}
