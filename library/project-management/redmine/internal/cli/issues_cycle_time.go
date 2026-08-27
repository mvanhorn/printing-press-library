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

type cycleTimeRow struct {
	Group       string  `json:"group"`
	ClosedCount int     `json:"closed_count"`
	AvgDays     float64 `json:"avg_days_to_close"`
}

func newNovelIssuesCycleTimeCmd(flags *rootFlags) *cobra.Command {
	var flagGroupBy string
	var flagProject string

	cmd := &cobra.Command{
		Use:     "cycle-time",
		Short:   "Average days from creation to close, grouped by tracker or project — real duration math the API can't do.",
		Long:    "Use this for duration-based aggregation (average days to close). Do NOT use the generic 'analytics' command for this — it only counts rows per group, it does not compute durations.",
		Example: "  redmine-pp-cli issues cycle-time --group-by tracker --project demo --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "issues cycle-time")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("issues cycle-time has no live equivalent; drop --data-source live"))
			}
			if flagGroupBy != "tracker" && flagGroupBy != "project" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be %q or %q, got %q", "tracker", "project", flagGroupBy))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("redmine-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: redmine-pp-cli sync --resources issues-json --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]cycleTimeRow, 0), flags)
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

			groupField := "$.tracker.name"
			if flagGroupBy == "project" {
				groupField = "$.project.name"
			}

			query := fmt.Sprintf(`SELECT
					json_extract(data, '%s') AS group_key,
					COUNT(*) AS closed_count,
					AVG(julianday(json_extract(data, '$.closed_on')) - julianday(json_extract(data, '$.created_on'))) AS avg_days
				FROM resources
				WHERE resource_type = 'issues-json'
				  AND json_extract(data, '$.closed_on') IS NOT NULL`, groupField)
			queryArgs := []any{}
			if flagProject != "" {
				query += ` AND json_extract(data, '$.project.id') = ?`
				queryArgs = append(queryArgs, projectID)
			}
			query += ` GROUP BY group_key ORDER BY avg_days DESC`

			rows, err := db.DB().QueryContext(ctx, query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			results := make([]cycleTimeRow, 0)
			for rows.Next() {
				var row cycleTimeRow
				if err := rows.Scan(&row.Group, &row.ClosedCount, &row.AvgDays); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
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
				fmt.Fprintln(cmd.OutOrStdout(), "No closed issues found to compute cycle time.")
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d closed\t%.1f days avg\n", r.Group, r.ClosedCount, r.AvgDays)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "tracker", "Group by 'tracker' or 'project'")
	cmd.Flags().StringVar(&flagProject, "project", "", "Limit to one project (identifier or numeric id)")
	return cmd
}
