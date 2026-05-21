// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: FTS5 query across cached agent task text.
// Provides `ori-pp-cli tasks search "<query>"` at top level.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/store"
)

func newTasksRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Query the local SQLite mirror of cached agent tasks",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newTasksSearchCmd(flags))
	return cmd
}

func newTasksSearchCmd(flags *rootFlags) *cobra.Command {
	var agentFilter, stateFilter, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "FTS5 search across cached agent task transcripts",
		Long: `Full-text search across the accumulated text of every cached task
(populated by 'ori-pp-cli sync'). Filters narrow by agent, state, and recency.
Returns task_id, agent, state, and a snippet with the matched terms.`,
		Example: `  ori-pp-cli tasks search "kanban hygiene"
  ori-pp-cli tasks search nerve --agent ori --since 7d
  ori-pp-cli tasks search "approval gate" --state completed --limit 20 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
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
				if dur, derr := time.ParseDuration(since); derr == nil {
					sinceTS = time.Now().UTC().Add(-dur).Format(time.RFC3339)
				} else {
					return usageErr(fmt.Errorf("--since must be a Go duration (e.g. 24h, 7d wrong: use 168h): %v", derr))
				}
			}

			sql := `
				SELECT t.id, t.agent, t.context_id, t.state, t.state_timestamp,
					snippet(tasks_fts, 2, '[', ']', '...', 12) AS hit
				FROM tasks_fts
				JOIN tasks_cache t ON t.id = tasks_fts.id AND t.agent = tasks_fts.agent
				WHERE tasks_fts MATCH ?
			`
			args2 := []any{query}
			if agentFilter != "" {
				sql += " AND t.agent = ?"
				args2 = append(args2, agentFilter)
			}
			if stateFilter != "" {
				sql += " AND t.state = ?"
				args2 = append(args2, normalizeState(stateFilter))
			}
			if sinceTS != "" {
				sql += " AND t.state_timestamp >= ?"
				args2 = append(args2, sinceTS)
			}
			sql += " ORDER BY rank LIMIT ?"
			args2 = append(args2, limit)

			rows, qerr := s.DB().QueryContext(cmd.Context(), sql, args2...)
			if qerr != nil {
				return apiErr(fmt.Errorf("FTS5 query: %w", qerr))
			}
			defer rows.Close()
			type hit struct {
				ID        string `json:"id"`
				Agent     string `json:"agent"`
				ContextID string `json:"context_id,omitempty"`
				State     string `json:"state"`
				Timestamp string `json:"state_timestamp,omitempty"`
				Snippet   string `json:"snippet"`
			}
			var results []hit
			for rows.Next() {
				var h hit
				if err := rows.Scan(&h.ID, &h.Agent, &h.ContextID, &h.State, &h.Timestamp, &h.Snippet); err != nil {
					continue
				}
				h.State = shortenState(h.State)
				results = append(results, h)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"query":   query,
					"count":   len(results),
					"results": results,
				}, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(w, "No matches. Try 'ori-pp-cli sync' if the local mirror is stale.")
				return nil
			}
			for _, h := range results {
				short := h.ID
				if len(short) > 12 {
					short = short[:12]
				}
				fmt.Fprintf(w, "  %-4s  %s  %-12s  %s\n", h.Agent, short, h.State, h.Snippet)
				if h.ContextID != "" {
					fmt.Fprintf(w, "          context=%s  at=%s\n", h.ContextID, h.Timestamp)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Filter by agent name")
	cmd.Flags().StringVar(&stateFilter, "state", "", "Filter by terminal state (completed, failed, canceled, running, input_required)")
	cmd.Flags().StringVar(&since, "since", "", "Filter to tasks updated within this duration (e.g. 24h, 168h)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	return cmd
}

func normalizeState(s string) string {
	s = strings.ToUpper(s)
	if !strings.HasPrefix(s, "TASK_STATE_") {
		s = "TASK_STATE_" + s
	}
	return s
}
