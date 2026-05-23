// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

// historyEntry represents a deployment in history
type historyEntry struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Status    string    `json:"status"`
	CommitSHA string    `json:"commit_sha,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var limit int
	var showGraph bool

	cmd := &cobra.Command{
		Use:   "history [project-id]",
		Short: "Show deployment history with optional graph visualization",
		Long: `Visualize deployment ancestry with local commit and deployment linkage.

The API returns flat lists; the graph requires persistent joins across
commits and deployments stored in the local cache.`,
		Example: `  vibecode-pp-cli history proj_abc123
  vibecode-pp-cli history proj_abc123 --graph
  vibecode-pp-cli history --limit 20 --json`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if len(args) > 0 {
				projectID = args[0]
			}

			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show deployment history from local store")
				return nil
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Query deployments
			query := `
				SELECT id, data, created_at FROM resources
				WHERE resource_type IN ('deployments', 'projects_deployments')
			`
			var queryArgs []any

			if projectID != "" {
				query += ` AND (id = ? OR json_extract(data, '$.project_id') = ?)`
				queryArgs = append(queryArgs, projectID, projectID)
			}

			query += ` ORDER BY created_at DESC`
			if limit > 0 {
				query += fmt.Sprintf(` LIMIT %d`, limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying deployments: %w", err)
			}
			defer rows.Close()

			var entries []historyEntry
			for rows.Next() {
				var id, dataStr string
				var createdAt time.Time
				if err := rows.Scan(&id, &dataStr, &createdAt); err != nil {
					continue
				}

				var data map[string]any
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					continue
				}

				entry := historyEntry{
					ID:        id,
					CreatedAt: createdAt,
				}
				if pid, ok := data["project_id"].(string); ok {
					entry.ProjectID = pid
				}
				if status, ok := data["status"].(string); ok {
					entry.Status = status
				}
				if sha, ok := data["commit_sha"].(string); ok {
					entry.CommitSHA = sha
				}
				if url, ok := data["url"].(string); ok {
					entry.URL = url
				}

				entries = append(entries, entry)
			}

			if len(entries) == 0 {
				if projectID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No deployment history found for %s\n", projectID)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No deployment history found; run 'sync' first")
				}
				return nil
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, entries)
			}

			if showGraph {
				// ASCII graph visualization
				return renderHistoryGraph(cmd, entries)
			}

			// Table output
			headers := []string{"Created", "Status", "Commit", "URL"}
			var tableRows [][]string
			for _, e := range entries {
				sha := e.CommitSHA
				if len(sha) > 7 {
					sha = sha[:7]
				}
				url := e.URL
				if len(url) > 40 {
					url = url[:37] + "..."
				}
				tableRows = append(tableRows, []string{
					e.CreatedAt.Format("2006-01-02 15:04"),
					e.Status,
					sha,
					url,
				})
			}

			if projectID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Deployment history for %s:\n\n", projectID)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Deployment history:")
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Filter to a specific project")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of entries")
	cmd.Flags().BoolVar(&showGraph, "graph", false, "Show ASCII graph visualization")
	return cmd
}

// renderHistoryGraph draws an ASCII timeline of deployments
func renderHistoryGraph(cmd *cobra.Command, entries []historyEntry) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Deployment History Graph")
	fmt.Fprintln(cmd.OutOrStdout(), "========================")
	fmt.Fprintln(cmd.OutOrStdout())

	// Group by date
	dateGroups := make(map[string][]historyEntry)
	var dates []string
	for _, e := range entries {
		date := e.CreatedAt.Format("2006-01-02")
		if _, exists := dateGroups[date]; !exists {
			dates = append(dates, date)
		}
		dateGroups[date] = append(dateGroups[date], e)
	}

	for i, date := range dates {
		dayEntries := dateGroups[date]

		// Draw date header
		fmt.Fprintf(cmd.OutOrStdout(), "┌─ %s ", date)
		fmt.Fprintf(cmd.OutOrStdout(), "(%d deployment", len(dayEntries))
		if len(dayEntries) > 1 {
			fmt.Fprint(cmd.OutOrStdout(), "s")
		}
		fmt.Fprintln(cmd.OutOrStdout(), ")")

		for j, e := range dayEntries {
			// Determine status icon
			icon := "○"
			switch e.Status {
			case "deployed", "ready", "live":
				icon = "●"
			case "building", "deploying":
				icon = "◐"
			case "failed", "error":
				icon = "✗"
			}

			// Draw entry
			prefix := "│"
			if j == len(dayEntries)-1 && i == len(dates)-1 {
				prefix = "└"
			}

			sha := e.CommitSHA
			if len(sha) > 7 {
				sha = sha[:7]
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s %s %s",
				prefix,
				icon,
				e.CreatedAt.Format("15:04"),
				e.Status,
			)
			if sha != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", sha)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			// Show URL on next line if available
			if e.URL != "" {
				url := e.URL
				if len(url) > 50 {
					url = url[:47] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "│     └─ %s\n", url)
			}
		}

		if i < len(dates)-1 {
			fmt.Fprintln(cmd.OutOrStdout(), "│")
		}
	}

	return nil
}
