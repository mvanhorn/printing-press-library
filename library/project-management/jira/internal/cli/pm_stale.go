// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/jira/internal/store"
	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var project string
	var statusFilter string
	var issueType string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find issues with no updates in N days",
		Long: `Scan locally synced issues for items that have not been updated within
the specified number of days. Useful for identifying forgotten or zombie work.

Data must be synced first: run 'sync --project KEY'.`,
		Example: `  # Issues not updated in 14 days (default)
  jira-cloud-platform-pp-cli stale

  # Custom threshold
  jira-cloud-platform-pp-cli stale --days 7

  # In-Progress issues not touched in a week
  jira-cloud-platform-pp-cli stale --days 7 --status "In Progress"

  # Scoped to a project
  jira-cloud-platform-pp-cli stale --days 14 --project MYPROJ

  # JSON output for agents
  jira-cloud-platform-pp-cli stale --days 14 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jira-cloud-platform-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'jira-cloud-platform-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

			query := `
SELECT
  json_extract(data, '$.key') as issue_key,
  json_extract(data, '$.fields.summary') as summary,
  json_extract(data, '$.fields.status.name') as issue_status,
  json_extract(data, '$.fields.assignee.displayName') as assignee,
  json_extract(data, '$.fields.priority.name') as priority,
  json_extract(data, '$.fields.updated') as updated,
  json_extract(data, '$.fields.issuetype.name') as itype
FROM issue
WHERE json_extract(data, '$.fields.status.statusCategory.key') != 'done'
  AND substr(json_extract(data, '$.fields.updated'), 1, 10) < ?`

			qargs := []any{cutoff}

			if project != "" {
				query += ` AND json_extract(data, '$.fields.project.key') = ?`
				qargs = append(qargs, project)
			}
			if statusFilter != "" {
				query += ` AND json_extract(data, '$.fields.status.name') = ?`
				qargs = append(qargs, statusFilter)
			}
			if issueType != "" {
				types := strings.Split(issueType, ",")
				placeholders := make([]string, len(types))
				for i, t := range types {
					placeholders[i] = "?"
					qargs = append(qargs, strings.TrimSpace(t))
				}
				query += ` AND json_extract(data, '$.fields.issuetype.name') IN (` + strings.Join(placeholders, ",") + `)`
			}

			query += ` ORDER BY substr(json_extract(data, '$.fields.updated'), 1, 10) ASC LIMIT ?`
			qargs = append(qargs, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying stale issues: %w", err)
			}
			defer rows.Close()

			type staleIssue struct {
				Key       string `json:"key"`
				Summary   string `json:"summary"`
				Status    string `json:"status"`
				Assignee  string `json:"assignee"`
				Priority  string `json:"priority"`
				Updated   string `json:"updated"`
				Type      string `json:"type"`
				DaysSince int    `json:"days_since"`
			}

			var items []staleIssue
			now := time.Now()

			for rows.Next() {
				var key, summary, issueStatus, assignee, priority, updated, itype string
				if err := rows.Scan(&key, &summary, &issueStatus, &assignee, &priority, &updated, &itype); err != nil {
					continue
				}
				staleDays := days
				if t, err := parseJiraTime(updated); err == nil {
					staleDays = int(now.Sub(t).Hours() / 24)
				}
				items = append(items, staleIssue{
					Key:       key,
					Summary:   summary,
					Status:    issueStatus,
					Assignee:  assignee,
					Priority:  priority,
					Updated:   updated,
					Type:      itype,
					DaysSince: staleDays,
				})
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale issues found.")
				return nil
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}

			out := cmd.OutOrStdout()
			scope := "all projects"
			if project != "" {
				scope = project
			}
			fmt.Fprintf(out, "Stale Issues — %s (not updated in %d+ days)\n\n", scope, days)
			fmt.Fprintf(out, "%-14s %-8s %-20s %-16s %s\n", "Key", "Days", "Assignee", "Status", "Summary")
			fmt.Fprintf(out, "%s\n", strings.Repeat("-", 80))
			for _, r := range items {
				summ := r.Summary
				if len(summ) > 40 {
					summ = summ[:37] + "..."
				}
				assigneeDisplay := r.Assignee
				if assigneeDisplay == "" {
					assigneeDisplay = "(unassigned)"
				}
				if len(assigneeDisplay) > 18 {
					assigneeDisplay = assigneeDisplay[:15] + "..."
				}
				statusDisplay := r.Status
				if len(statusDisplay) > 14 {
					statusDisplay = statusDisplay[:11] + "..."
				}
				fmt.Fprintf(out, "%-14s %-8d %-20s %-16s %s\n",
					r.Key, r.DaysSince, assigneeDisplay, statusDisplay, summ)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Days without update to consider stale")
	cmd.Flags().StringVar(&project, "project", "", "Project key (e.g. MYPROJ)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status name (e.g. 'In Progress')")
	cmd.Flags().StringVar(&issueType, "type", "", "Filter by issue type, comma-separated (e.g. Task,Story)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum issues to show")

	return cmd
}
