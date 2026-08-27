// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/store"
)

type staleIssueRow struct {
	ID        int64  `json:"id"`
	Subject   string `json:"subject"`
	Project   string `json:"project"`
	Tracker   string `json:"tracker"`
	Status    string `json:"status"`
	Assignee  string `json:"assignee,omitempty"`
	UpdatedOn string `json:"updated_on"`
	DaysStale int    `json:"days_stale"`
}

func newNovelIssuesStaleCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagDays int

	cmd := &cobra.Command{
		Use:     "stale",
		Short:   "Find open issues nobody has touched in N days — Redmine has no built-in staleness concept.",
		Long:    "Use this to find issues that have gone quiet (no recent activity). Do NOT use it for recently changed issues; use 'digest' instead — its semantics are the opposite (recent activity, not inactivity).",
		Example: "  redmine-pp-cli issues stale --project demo --days 14 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "issues stale")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("issues stale has no live equivalent; drop --data-source live"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("redmine-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: redmine-pp-cli sync --resources issues-json --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]staleIssueRow, 0), flags)
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

			cutoff := time.Now().UTC().AddDate(0, 0, -flagDays).Format(time.RFC3339)

			query := `SELECT
					json_extract(data, '$.id'),
					json_extract(data, '$.subject'),
					json_extract(data, '$.project.name'),
					json_extract(data, '$.tracker.name'),
					json_extract(data, '$.status.name'),
					json_extract(data, '$.assigned_to.name'),
					json_extract(data, '$.updated_on')
				FROM resources
				WHERE resource_type = 'issues-json'
				  AND json_extract(data, '$.status.is_closed') = 0
				  AND json_extract(data, '$.updated_on') < ?`
			queryArgs := []any{cutoff}
			if flagProject != "" {
				query += ` AND json_extract(data, '$.project.id') = ?`
				queryArgs = append(queryArgs, projectID)
			}
			query += ` ORDER BY json_extract(data, '$.updated_on') ASC`

			rows, err := db.DB().QueryContext(ctx, query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			results := make([]staleIssueRow, 0)
			now := time.Now().UTC()
			for rows.Next() {
				var row staleIssueRow
				var assignee sql.NullString
				var updatedOn string
				if err := rows.Scan(&row.ID, &row.Subject, &row.Project, &row.Tracker, &row.Status, &assignee, &updatedOn); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
				}
				row.Assignee = assignee.String
				row.UpdatedOn = updatedOn
				if t, err := time.Parse(time.RFC3339, updatedOn); err == nil {
					row.DaysStale = int(now.Sub(t).Hours() / 24)
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
				fmt.Fprintf(cmd.OutOrStdout(), "No stale issues found (older than %d days).\n", flagDays)
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d\t%s\t%s\t%s\t%dd stale\t%s\n", r.ID, r.Project, r.Status, r.Tracker, r.DaysStale, r.Subject)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Limit to one project (identifier or numeric id)")
	cmd.Flags().IntVar(&flagDays, "days", 14, "Minimum days since last update to count as stale")
	return cmd
}
