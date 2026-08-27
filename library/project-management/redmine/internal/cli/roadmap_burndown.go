// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/store"
)

type burndownView struct {
	Version        string  `json:"version"`
	Project        string  `json:"project"`
	Status         string  `json:"status,omitempty"`
	DueDate        string  `json:"due_date,omitempty"`
	OpenIssues     int     `json:"open_issues"`
	ClosedIssues   int     `json:"closed_issues"`
	TotalIssues    int     `json:"total_issues"`
	AvgDoneRatio   float64 `json:"avg_done_ratio"`
	EstimatedHours float64 `json:"estimated_hours"`
	LoggedHours    float64 `json:"logged_hours"`
	HoursVariance  float64 `json:"hours_variance"`
	ReadyToClose   bool    `json:"ready_to_close"`
}

func newNovelRoadmapBurndownCmd(flags *rootFlags) *cobra.Command {
	var flagProject string

	cmd := &cobra.Command{
		Use:     "burndown <version>",
		Short:   "See open/closed issue counts, average completion, and logged-vs-estimated hours for a release version — the Roadmap page Redmine never exposed as an API.",
		Long:    "Use this for version/roadmap progress and close-readiness. Do NOT use it for a single issue's status; use 'issues get' instead.",
		Example: "  redmine-pp-cli roadmap burndown 1.0 --project demo --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "version=1.0;--project=demo",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "roadmap burndown")
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("version is required\nUsage: %s <version>", cmd.CommandPath()))
			}
			if flagProject == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project is required (Redmine versions are scoped to a project)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			projBody, err := c.Get(ctx, replacePathParam("/projects/{project_id}.json", "project_id", flagProject), nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var projWrap struct {
				Project struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
			}
			if err := json.Unmarshal(projBody, &projWrap); err != nil {
				return fmt.Errorf("parsing project %q: %w", flagProject, err)
			}

			versBody, err := c.Get(ctx, replacePathParam("/projects/{project_id}/versions.json", "project_id", flagProject), nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var versWrap struct {
				Versions []struct {
					ID      int64  `json:"id"`
					Name    string `json:"name"`
					Status  string `json:"status"`
					DueDate string `json:"due_date"`
				} `json:"versions"`
			}
			if err := json.Unmarshal(versBody, &versWrap); err != nil {
				return fmt.Errorf("parsing versions for project %q: %w", flagProject, err)
			}

			target := strings.TrimSpace(args[0])
			var matched *struct {
				ID      int64  `json:"id"`
				Name    string `json:"name"`
				Status  string `json:"status"`
				DueDate string `json:"due_date"`
			}
			for i := range versWrap.Versions {
				v := &versWrap.Versions[i]
				if v.Name == target || strconv.FormatInt(v.ID, 10) == target {
					matched = v
					break
				}
			}
			if matched == nil {
				available := make([]string, 0, len(versWrap.Versions))
				for _, v := range versWrap.Versions {
					available = append(available, v.Name)
				}
				msg := fmt.Sprintf("version %q not found in project %q", target, projWrap.Project.Name)
				if len(available) > 0 {
					msg += fmt.Sprintf(" (available: %s)", strings.Join(available, ", "))
				} else {
					msg += " (project has no versions)"
				}
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error":              msg,
						"available_versions": available,
					}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(fmt.Errorf("%s", msg))
			}

			dbPath := defaultDBPath("redmine-pp-cli")
			view := burndownView{
				Version: matched.Name,
				Project: projWrap.Project.Name,
				Status:  matched.Status,
				DueDate: matched.DueDate,
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: redmine-pp-cli sync --resources issues-json,time-entries-json --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT
					json_extract(data, '$.id'),
					json_extract(data, '$.status.is_closed'),
					COALESCE(json_extract(data, '$.done_ratio'), 0),
					COALESCE(json_extract(data, '$.estimated_hours'), 0)
				FROM resources
				WHERE resource_type = 'issues-json'
				  AND json_extract(data, '$.project.id') = ?
				  AND json_extract(data, '$.fixed_version.id') = ?`, projWrap.Project.ID, matched.ID)
			if err != nil {
				return fmt.Errorf("query issues: %w", err)
			}
			issueIDs := make([]int64, 0)
			var doneRatioSum float64
			for rows.Next() {
				var id int64
				var isClosed int
				var doneRatio, estHours float64
				if err := rows.Scan(&id, &isClosed, &doneRatio, &estHours); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan issue row: %w", err)
				}
				issueIDs = append(issueIDs, id)
				if isClosed != 0 {
					view.ClosedIssues++
				} else {
					view.OpenIssues++
				}
				doneRatioSum += doneRatio
				view.EstimatedHours += estHours
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate issue rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close issue rows: %w", err)
			}
			view.TotalIssues = view.OpenIssues + view.ClosedIssues
			if view.TotalIssues > 0 {
				view.AvgDoneRatio = doneRatioSum / float64(view.TotalIssues)
			}
			view.ReadyToClose = view.TotalIssues > 0 && view.OpenIssues == 0

			if len(issueIDs) > 0 {
				placeholders := make([]string, len(issueIDs))
				teArgs := make([]any, len(issueIDs))
				for i, id := range issueIDs {
					placeholders[i] = "?"
					teArgs[i] = id
				}
				teQuery := fmt.Sprintf(`SELECT COALESCE(SUM(json_extract(data, '$.hours')), 0)
					FROM resources
					WHERE resource_type = 'time-entries-json'
					  AND json_extract(data, '$.issue.id') IN (%s)`, strings.Join(placeholders, ","))
				if err := db.DB().QueryRowContext(ctx, teQuery, teArgs...).Scan(&view.LoggedHours); err != nil {
					return fmt.Errorf("query time entries: %w", err)
				}
			}
			view.HoursVariance = view.LoggedHours - view.EstimatedHours

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s / %s (%s)\n", view.Project, view.Version, view.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  %d open, %d closed (%d total), avg done_ratio %.0f%%\n", view.OpenIssues, view.ClosedIssues, view.TotalIssues, view.AvgDoneRatio)
			fmt.Fprintf(cmd.OutOrStdout(), "  %.1fh estimated, %.1fh logged (variance %.1fh)\n", view.EstimatedHours, view.LoggedHours, view.HoursVariance)
			if view.ReadyToClose {
				fmt.Fprintln(cmd.OutOrStdout(), "  Ready to close: no open issues remain.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Project identifier or numeric id (required)")
	return cmd
}
