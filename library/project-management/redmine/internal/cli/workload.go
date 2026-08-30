// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/store"
)

type workloadRow struct {
	AssigneeID     int64   `json:"assignee_id"`
	Assignee       string  `json:"assignee"`
	OpenIssues     int     `json:"open_issues"`
	EstimatedHours float64 `json:"estimated_hours"`
	OverThreshold  bool    `json:"over_threshold,omitempty"`
}

func newNovelWorkloadCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagThreshold int

	cmd := &cobra.Command{
		Use:     "workload",
		Short:   "See every assignee's open-issue count and estimated hours in one aggregate view, with a flag for anyone over threshold.",
		Long:    "Use this for an aggregate view across all assignees. Do NOT use it to see one person's issue list; use 'issues-json get-issues --assigned-to-id <user>' instead.",
		Example: "  redmine-pp-cli workload --project demo --threshold 5 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workload")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("workload has no live equivalent; drop --data-source live"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("redmine-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: redmine-pp-cli sync --resources issues-json --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]workloadRow, 0), flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var projectID int64
			if flagProject != "" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				body, err := c.Get(ctx, replacePathParam("/projects/{project_id}.json", "project_id", flagProject), nil)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var wrap struct {
					Project struct {
						ID int64 `json:"id"`
					} `json:"project"`
				}
				if err := json.Unmarshal(body, &wrap); err != nil {
					return fmt.Errorf("parsing project %q: %w", flagProject, err)
				}
				projectID = wrap.Project.ID
			}

			query := `SELECT
					COALESCE(json_extract(data, '$.assigned_to.id'), -1) AS assignee_id,
					COALESCE(json_extract(data, '$.assigned_to.name'), 'Unassigned') AS assignee_name,
					COUNT(*) AS open_count,
					COALESCE(SUM(json_extract(data, '$.estimated_hours')), 0) AS estimated_hours
				FROM resources
				WHERE resource_type = 'issues-json'
				  AND json_extract(data, '$.status.is_closed') = 0`
			queryArgs := []any{}
			if flagProject != "" {
				query += ` AND json_extract(data, '$.project.id') = ?`
				queryArgs = append(queryArgs, projectID)
			}
			query += ` GROUP BY assignee_id ORDER BY open_count DESC`

			rows, err := db.DB().QueryContext(ctx, query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			results := make([]workloadRow, 0)
			for rows.Next() {
				var row workloadRow
				if err := rows.Scan(&row.AssigneeID, &row.Assignee, &row.OpenIssues, &row.EstimatedHours); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
				}
				if flagThreshold > 0 && row.OpenIssues >= flagThreshold {
					row.OverThreshold = true
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close rows: %w", err)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open issues found.")
				return nil
			}
			for _, r := range results {
				flag := ""
				if r.OverThreshold {
					flag = "  ** over threshold **"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d open\t%.1fh estimated%s\n", r.Assignee, r.OpenIssues, r.EstimatedHours, flag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Limit to one project (identifier or numeric id)")
	cmd.Flags().IntVar(&flagThreshold, "threshold", 0, "Flag assignees at or above this many open issues (0 disables flagging)")
	return cmd
}
