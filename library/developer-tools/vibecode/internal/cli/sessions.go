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

func newSessionsCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var limit int
	var since string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Track agent command sessions",
		Long: `Group sequential agent commands by session to understand what Claude Code
or Cursor did during a work session.

Sessions are inferred from locally-cached agent command logs based on timing
proximity. Commands within 30 minutes of each other are grouped into the
same session. Requires synced data - run 'sync' first.`,
		Example: `  vibecode-pp-cli sessions
  vibecode-pp-cli sessions --project proj_abc123
  vibecode-pp-cli sessions --since "2 hours ago" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for agent sessions")
				return nil
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Build query
			query := `
				SELECT id, data, created_at FROM resources
				WHERE resource_type IN ('agent_commands', 'agent_sessions')
			`
			var queryArgs []any

			if projectID != "" {
				query += ` AND json_extract(data, '$.project_id') = ?`
				queryArgs = append(queryArgs, projectID)
			}

			if since != "" {
				cutoff, err := parseTimeExpression(since)
				if err != nil {
					return fmt.Errorf("invalid --since value: %w", err)
				}
				query += ` AND created_at >= ?`
				queryArgs = append(queryArgs, cutoff.Format(time.RFC3339))
			}

			query += ` ORDER BY created_at DESC`
			if limit > 0 {
				query += fmt.Sprintf(` LIMIT %d`, limit*10) // Get more to allow grouping
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying agent commands: %w", err)
			}
			defer rows.Close()

			type agentCommand struct {
				ID        string    `json:"id"`
				ProjectID string    `json:"project_id"`
				Prompt    string    `json:"prompt"`
				Model     string    `json:"model"`
				Status    string    `json:"status"`
				CreatedAt time.Time `json:"created_at"`
			}

			var commands []agentCommand
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

				ac := agentCommand{
					ID:        id,
					CreatedAt: createdAt,
				}
				if pid, ok := data["project_id"].(string); ok {
					ac.ProjectID = pid
				}
				if prompt, ok := data["prompt"].(string); ok {
					ac.Prompt = prompt
				}
				if model, ok := data["model"].(string); ok {
					ac.Model = model
				}
				if status, ok := data["status"].(string); ok {
					ac.Status = status
				}

				commands = append(commands, ac)
			}

			if len(commands) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agent commands found in local cache")
				fmt.Fprintln(cmd.OutOrStdout(), "Agent commands are recorded when you use 'agent send' or 'yolo'")
				return nil
			}

			// Group commands into sessions (commands within 30 min of each other)
			const sessionGap = 30 * time.Minute

			type session struct {
				ID           string         `json:"id"`
				ProjectID    string         `json:"project_id,omitempty"`
				StartTime    time.Time      `json:"start_time"`
				EndTime      time.Time      `json:"end_time"`
				Duration     string         `json:"duration"`
				Commands     []agentCommand `json:"commands"`
				CommandCount int            `json:"command_count"`
			}

			var sessions []session
			var currentSession *session

			// Commands are in DESC order, so reverse for chronological processing
			for i := len(commands) - 1; i >= 0; i-- {
				cmd := commands[i]

				if currentSession == nil {
					currentSession = &session{
						ID:        fmt.Sprintf("session_%d", len(sessions)+1),
						ProjectID: cmd.ProjectID,
						StartTime: cmd.CreatedAt,
						EndTime:   cmd.CreatedAt,
						Commands:  []agentCommand{cmd},
					}
					continue
				}

				// Check if this command belongs to the current session
				if cmd.CreatedAt.Sub(currentSession.EndTime) <= sessionGap {
					currentSession.Commands = append(currentSession.Commands, cmd)
					currentSession.EndTime = cmd.CreatedAt
					if cmd.ProjectID != "" && currentSession.ProjectID == "" {
						currentSession.ProjectID = cmd.ProjectID
					}
				} else {
					// Finalize current session
					currentSession.CommandCount = len(currentSession.Commands)
					currentSession.Duration = currentSession.EndTime.Sub(currentSession.StartTime).Round(time.Minute).String()
					sessions = append(sessions, *currentSession)

					// Start new session
					currentSession = &session{
						ID:        fmt.Sprintf("session_%d", len(sessions)+1),
						ProjectID: cmd.ProjectID,
						StartTime: cmd.CreatedAt,
						EndTime:   cmd.CreatedAt,
						Commands:  []agentCommand{cmd},
					}
				}
			}

			// Finalize last session
			if currentSession != nil {
				currentSession.CommandCount = len(currentSession.Commands)
				currentSession.Duration = currentSession.EndTime.Sub(currentSession.StartTime).Round(time.Minute).String()
				sessions = append(sessions, *currentSession)
			}

			// Reverse to get most recent first
			for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}

			// Apply limit to sessions
			if limit > 0 && len(sessions) > limit {
				sessions = sessions[:limit]
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, sessions)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d agent sessions\n\n", len(sessions))

			for _, s := range sessions {
				fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", s.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "  Started: %s\n", s.StartTime.Format("2006-01-02 15:04"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %s\n", s.Duration)
				fmt.Fprintf(cmd.OutOrStdout(), "  Commands: %d\n", s.CommandCount)
				if s.ProjectID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Project: %s\n", s.ProjectID)
				}

				// Show first few prompts
				for i, c := range s.Commands {
					if i >= 3 {
						fmt.Fprintf(cmd.OutOrStdout(), "    ... and %d more\n", len(s.Commands)-3)
						break
					}
					prompt := c.Prompt
					if len(prompt) > 60 {
						prompt = prompt[:57] + "..."
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", prompt)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Filter to a specific project")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of sessions to show")
	cmd.Flags().StringVar(&since, "since", "", "Show sessions since time (e.g. '2 hours ago')")
	return cmd
}
