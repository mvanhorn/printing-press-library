// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/store"
)

type executionExportRow struct {
	ID          string     `json:"id"`
	WorkflowID  string     `json:"workflow_id"`
	Status      string     `json:"status"`
	Mode        string     `json:"mode"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
	DurationSec float64    `json:"duration_sec,omitempty"`
	RetryOf     string     `json:"retry_of,omitempty"`
	WaitTill    *time.Time `json:"wait_till,omitempty"`
}

func newExecutionsExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var workflowID string
	var statusFilter string
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export structured execution history from the local store",
		Long: `Export execution records from the local SQLite store as structured JSON.
Supports filtering by workflow, status, and time window. Suitable for
piping into data pipelines, audit logs, or analytics tools.
Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Export all executions
  n8n-pp-cli executions export --json

  # Export failures for a specific workflow in the last 7 days
  n8n-pp-cli executions export --workflow abc123 --status error --since 7d --json

  # Export last 100 executions sorted by start time
  n8n-pp-cli executions export --limit 100 --json --agent`, "\n"),
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

			var sinceTime time.Time
			if since != "" {
				sinceTime, err = parseDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: use format like 7d, 24h, 30m", since))
				}
			}

			query := `SELECT id, workflow_id, status, mode, started_at, stopped_at, retry_of, wait_till
			            FROM executions WHERE 1=1`
			var qargs []any
			if workflowID != "" {
				query += " AND workflow_id = ?"
				qargs = append(qargs, workflowID)
			}
			if statusFilter != "" {
				query += " AND status = ?"
				qargs = append(qargs, statusFilter)
			}
			if !sinceTime.IsZero() {
				query += " AND started_at >= ?"
				qargs = append(qargs, sinceTime.Format(time.RFC3339))
			}
			query += " ORDER BY started_at DESC"
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying executions: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer rows.Close()

			var results []executionExportRow
			for rows.Next() {
				var r executionExportRow
				var startedStr, stoppedStr, waitStr, retryOf *string
				if err := rows.Scan(&r.ID, &r.WorkflowID, &r.Status, &r.Mode,
					&startedStr, &stoppedStr, &retryOf, &waitStr); err != nil {
					continue
				}
				if retryOf != nil {
					r.RetryOf = *retryOf
				}
				if startedStr != nil {
					if t, err := time.Parse(time.RFC3339, *startedStr); err == nil {
						r.StartedAt = &t
					}
				}
				if stoppedStr != nil {
					if t, err := time.Parse(time.RFC3339, *stoppedStr); err == nil {
						r.StoppedAt = &t
						if r.StartedAt != nil {
							r.DurationSec = t.Sub(*r.StartedAt).Seconds()
						}
					}
				}
				if waitStr != nil {
					if t, err := time.Parse(time.RFC3339, *waitStr); err == nil {
						r.WaitTill = &t
					}
				}
				results = append(results, r)
			}
			if rows.Err() != nil {
				return fmt.Errorf("scanning rows: %w", rows.Err())
			}

			if len(results) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No executions matched. Run 'n8n-pp-cli sync' first.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&workflowID, "workflow", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (success, error, waiting, running)")
	cmd.Flags().StringVar(&since, "since", "", "Only include executions started within this window (e.g. 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of rows to export (0 = all)")
	return cmd
}

// parseDuration parses a human-friendly duration like "7d", "24h", "30m" into a
// time.Time representing `now minus duration`.
func parseDuration(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty duration")
	}
	unit := s[len(s)-1]
	value := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		return time.Time{}, fmt.Errorf("cannot parse %q", s)
	}
	now := time.Now().UTC()
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'm':
		return now.Add(-time.Duration(n) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q in %q; use d, h, or m", string(unit), s)
	}
}
