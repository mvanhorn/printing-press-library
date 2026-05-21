// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: group cached tasks by context_id to surface
// conversations. Provides `ori-pp-cli contexts list` at top level.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/store"
)

func newContextsRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contexts",
		Short: "Group cached tasks by conversation context",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newContextsListCmd(flags))
	return cmd
}

func newContextsListCmd(flags *rootFlags) *cobra.Command {
	var agentFilter, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversation contexts derived from cached tasks",
		Long: `Groups cached tasks by (agent, context_id) and surfaces conversation-level
information: first/last activity timestamps, task count, and a peek of the
last assistant artifact text.

Useful for resuming a forgotten thread by context_id when you don't remember
a specific task_id from earlier work.`,
		Example: `  ori-pp-cli contexts list
  ori-pp-cli contexts list --agent ori --since 24h
  ori-pp-cli contexts list --since 168h --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ori-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer s.Close()
			if err := ensureTasksCacheSchema(s.DB()); err != nil {
				return apiErr(err)
			}

			sinceTS := ""
			if since != "" {
				dur, derr := time.ParseDuration(since)
				if derr != nil {
					return usageErr(fmt.Errorf("--since must be a Go duration (e.g. 24h, 168h): %v", derr))
				}
				sinceTS = time.Now().UTC().Add(-dur).Format(time.RFC3339)
			}

			sqlStr := `
				SELECT context_id, agent, COUNT(*) AS task_count,
					MIN(state_timestamp) AS first_at,
					MAX(state_timestamp) AS last_at,
					(SELECT text FROM tasks_cache t2
						WHERE t2.agent = t1.agent AND t2.context_id = t1.context_id
						ORDER BY state_timestamp ASC LIMIT 1) AS first_text
				FROM tasks_cache t1
				WHERE context_id != ''`
			args2 := []any{}
			if agentFilter != "" {
				sqlStr += " AND agent = ?"
				args2 = append(args2, agentFilter)
			}
			if sinceTS != "" {
				sqlStr += " AND state_timestamp >= ?"
				args2 = append(args2, sinceTS)
			}
			sqlStr += " GROUP BY agent, context_id ORDER BY last_at DESC LIMIT ?"
			args2 = append(args2, limit)

			rows, qerr := s.DB().QueryContext(cmd.Context(), sqlStr, args2...)
			if qerr != nil {
				return apiErr(fmt.Errorf("contexts query: %w", qerr))
			}
			defer rows.Close()
			type ctxRow struct {
				ContextID string `json:"context_id"`
				Agent     string `json:"agent"`
				TaskCount int    `json:"task_count"`
				FirstAt   string `json:"first_at,omitempty"`
				LastAt    string `json:"last_at,omitempty"`
				Peek      string `json:"first_text_peek,omitempty"`
			}
			var results []ctxRow
			for rows.Next() {
				var r ctxRow
				var peek sql.NullString
				if err := rows.Scan(&r.ContextID, &r.Agent, &r.TaskCount, &r.FirstAt, &r.LastAt, &peek); err != nil {
					continue
				}
				if peek.Valid {
					r.Peek = peekStr(peek.String, 100)
				}
				results = append(results, r)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"count":    len(results),
					"contexts": results,
				}, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(w, "No contexts found. Try 'ori-pp-cli sync' first.")
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(w, "  %-4s  %s  (%d tasks, last %s)\n", r.Agent, r.ContextID, r.TaskCount, r.LastAt)
				if r.Peek != "" {
					fmt.Fprintf(w, "        peek: %s\n", r.Peek)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Filter by agent name")
	cmd.Flags().StringVar(&since, "since", "", "Filter to contexts active within this duration (e.g. 24h, 168h)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max contexts to return")
	return cmd
}

func peekStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
