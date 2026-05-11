// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/jira/internal/store"
	"github.com/spf13/cobra"
)

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var projects string
	var issueType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Show open issue count and story point load per assignee",
		Long: `Aggregate open issues by assignee across one or more projects.
Shows issue count and story points per person so you can see who has capacity
before assigning new work.

Data must be synced first: run 'sync --project KEY'.`,
		Example: `  # Workload across all synced issues
  jira-cloud-platform-pp-cli workload

  # Scope to one project
  jira-cloud-platform-pp-cli workload --project MYPROJ

  # Cross-project: multiple projects
  jira-cloud-platform-pp-cli workload --project PROJ1,PROJ2

  # JSON output for agents
  jira-cloud-platform-pp-cli workload --project MYPROJ --agent`,
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

			query := `
SELECT
  json_extract(data, '$.fields.assignee.displayName') as assignee_name,
  json_extract(data, '$.fields.assignee.accountId') as account_id,
  COUNT(*) as open_issues,
  SUM(COALESCE(
    json_extract(data, '$.fields.story_points'),
    CAST(json_extract(data, '$.fields.customfield_10016') AS REAL),
    CAST(json_extract(data, '$.fields.customfield_10028') AS REAL),
    0
  )) as story_points
FROM issue
WHERE json_extract(data, '$.fields.status.statusCategory.key') != 'done'
  AND json_extract(data, '$.fields.assignee') IS NOT NULL`

			var qargs []any

			if issueType != "" {
				types := strings.Split(issueType, ",")
				placeholders := make([]string, len(types))
				for i, t := range types {
					placeholders[i] = "?"
					qargs = append(qargs, strings.TrimSpace(t))
				}
				query += ` AND json_extract(data, '$.fields.issuetype.name') IN (` + strings.Join(placeholders, ",") + `)`
			}

			if projects != "" {
				keys := strings.Split(projects, ",")
				placeholders := make([]string, len(keys))
				for i, k := range keys {
					placeholders[i] = "?"
					qargs = append(qargs, strings.TrimSpace(k))
				}
				query += ` AND json_extract(data, '$.fields.project.key') IN (` + strings.Join(placeholders, ",") + `)`
			}

			query += ` GROUP BY json_extract(data, '$.fields.assignee.accountId')
ORDER BY open_issues DESC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying workload: %w", err)
			}
			defer rows.Close()

			type workloadRow struct {
				Assignee    string  `json:"assignee"`
				AccountID   string  `json:"account_id"`
				OpenIssues  int     `json:"open_issues"`
				StoryPoints float64 `json:"story_points"`
			}

			var results []workloadRow
			for rows.Next() {
				var assignee, accountID string
				var openIssues int
				var storyPoints float64
				if err := rows.Scan(&assignee, &accountID, &openIssues, &storyPoints); err != nil {
					continue
				}
				if assignee == "" {
					assignee = "(unassigned)"
				}
				results = append(results, workloadRow{
					Assignee:    assignee,
					AccountID:   accountID,
					OpenIssues:  openIssues,
					StoryPoints: storyPoints,
				})
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open assigned issues found. Run 'sync --project KEY' first.")
				return nil
			}

			// Sort by open issues desc (already sorted by SQL, but ensure)
			sort.Slice(results, func(i, j int) bool {
				return results[i].OpenIssues > results[j].OpenIssues
			})

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			out := cmd.OutOrStdout()
			label := "all synced projects"
			if projects != "" {
				label = projects
			}
			fmt.Fprintf(out, "Assignee Workload — %s\n\n", label)
			fmt.Fprintf(out, "%-30s %8s %8s\n", "Assignee", "Issues", "Points")
			fmt.Fprintf(out, "%-30s %8s %8s\n", strings.Repeat("-", 30), "------", "------")
			for _, r := range results {
				pts := ""
				if r.StoryPoints > 0 {
					pts = fmt.Sprintf("%.0f", r.StoryPoints)
				} else {
					pts = "-"
				}
				fmt.Fprintf(out, "%-30s %8d %8s\n", r.Assignee, r.OpenIssues, pts)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projects, "project", "", "Project key(s), comma-separated (e.g. PROJ1,PROJ2)")
	cmd.Flags().StringVar(&issueType, "type", "", "Filter by issue type, comma-separated (e.g. Task,Story)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
