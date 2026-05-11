// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/jira/internal/store"
	"github.com/spf13/cobra"
)

// PATCH: novel-features
func newBlockedCmd(flags *rootFlags) *cobra.Command {
	var project string
	var assignee string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "Show issues that are blocked, with their blocker chain",
		Long: `Find open issues that are blocked by other issues.
Uses the local issue link store to traverse blocker relationships.

Data must be synced first: run 'sync --project KEY'.`,
		Example: `  # All blocked issues in local store
  jira-cloud-platform-pp-cli blocked

  # Scoped to a project
  jira-cloud-platform-pp-cli blocked --project MYPROJ

  # Blocked issues assigned to me
  jira-cloud-platform-pp-cli blocked --assignee me

  # JSON output for agents
  jira-cloud-platform-pp-cli blocked --project MYPROJ --agent`,
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

			// Get current user's display name if --assignee me
			resolvedAssignee := assignee
			if assignee == "me" {
				row := db.DB().QueryRowContext(cmd.Context(), `
					SELECT json_extract(data, '$.displayName')
					FROM resources WHERE resource_type = 'myself' LIMIT 1`)
				var name string
				if err := row.Scan(&name); err == nil && name != "" {
					resolvedAssignee = name
				}
			}

			// Query issues that have "is blocked by" links stored in the issue JSON
			// Jira stores issuelinks inside the issue data as fields.issuelinks[]
			// Each link has: type.inward ("is blocked by"), inwardIssue/outwardIssue
			//
			// Strategy: query issue table, filter where issuelinks JSON contains a
			// blocker-type link (inward text contains "block"), extract info.
			query := `
SELECT
  json_extract(data, '$.key') as issue_key,
  json_extract(data, '$.fields.summary') as summary,
  json_extract(data, '$.fields.status.name') as status,
  json_extract(data, '$.fields.assignee.displayName') as assignee,
  json_extract(data, '$.fields.priority.name') as priority,
  json_extract(data, '$.fields.issuelinks') as links_json
FROM issue
WHERE json_extract(data, '$.fields.status.statusCategory.key') != 'done'
  AND json_extract(data, '$.fields.issuelinks') IS NOT NULL
  AND (
    json_extract(data, '$.fields.issuelinks') LIKE '%blocked by%'
    OR json_extract(data, '$.fields.issuelinks') LIKE '%is blocked%'
    OR json_extract(data, '$.fields.issuelinks') LIKE '%Blocks%'
  )`

			var qargs []any
			if project != "" {
				query += ` AND json_extract(data, '$.fields.project.key') = ?`
				qargs = append(qargs, project)
			}
			if resolvedAssignee != "" && resolvedAssignee != "me" {
				query += ` AND json_extract(data, '$.fields.assignee.displayName') = ?`
				qargs = append(qargs, resolvedAssignee)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying blocked issues: %w", err)
			}
			defer rows.Close()

			type blockerRef struct {
				Key     string `json:"key"`
				Summary string `json:"summary"`
			}
			type blockedIssue struct {
				Key      string       `json:"key"`
				Summary  string       `json:"summary"`
				Status   string       `json:"status"`
				Assignee string       `json:"assignee"`
				Priority string       `json:"priority"`
				Blockers []blockerRef `json:"blockers"`
			}

			var results []blockedIssue

			for rows.Next() {
				var key, summary, status, assigneeName, priority, linksJSON string
				if err := rows.Scan(&key, &summary, &status, &assigneeName, &priority, &linksJSON); err != nil {
					continue
				}

				// Parse the issuelinks JSON to find actual blockers
				var links []map[string]any
				if err := json.Unmarshal([]byte(linksJSON), &links); err != nil {
					continue
				}

				var blockers []blockerRef
				for _, link := range links {
					linkType, _ := link["type"].(map[string]any)
					if linkType == nil {
						continue
					}
					inwardText, _ := linkType["inward"].(string)
					outwardText, _ := linkType["outward"].(string)

					// "is blocked by" — the current issue is the inwardIssue
					if strings.Contains(strings.ToLower(inwardText), "blocked by") ||
						strings.Contains(strings.ToLower(inwardText), "is blocked") {
						if inwardIssue, ok := link["inwardIssue"].(map[string]any); ok {
							blockerKey, _ := inwardIssue["key"].(string)
							var blockerSummary string
							if fields, ok := inwardIssue["fields"].(map[string]any); ok {
								blockerSummary, _ = fields["summary"].(string)
							}
							blockers = append(blockers, blockerRef{Key: blockerKey, Summary: blockerSummary})
						}
					}
					// "blocks" outward — check outwardIssue but that means THIS issue blocks another
					_ = outwardText
				}

				if len(blockers) == 0 {
					continue
				}

				results = append(results, blockedIssue{
					Key:      key,
					Summary:  summary,
					Status:   status,
					Assignee: assigneeName,
					Priority: priority,
					Blockers: blockers,
				})
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No blocked issues found.")
				return nil
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Blocked Issues — %d found\n\n", len(results))
			for _, r := range results {
				fmt.Fprintf(out, "  %s  [%s]  %s\n", r.Key, r.Status, r.Summary)
				for _, b := range r.Blockers {
					blockerLine := "    ↳ blocked by: " + b.Key
					if b.Summary != "" {
						blockerLine += "  " + b.Summary
					}
					fmt.Fprintln(out, blockerLine)
				}
				if r.Assignee != "" {
					fmt.Fprintf(out, "    Assignee: %s  Priority: %s\n", r.Assignee, r.Priority)
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project key (e.g. MYPROJ)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee display name, or 'me' for current user")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
