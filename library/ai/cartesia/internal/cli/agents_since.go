// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

type feedEntry struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Timestamp string  `json:"timestamp"`
	MetricID  string  `json:"metric_id,omitempty"`
	Score     float64 `json:"score,omitempty"`
	Status    string  `json:"status,omitempty"`
	Summary   string  `json:"summary,omitempty"`
}

// newAgentsSinceCmd registers `agents since <agent-id> <when>` — emits a
// chronological feed of calls + failing metric results for an agent.
func newAgentsSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "since [agent-id] [when]",
		Short:       "Chronological feed of calls and failing metric results for an agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli agents since agent_abc 24h --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			agentID := args[0]
			when := args[1]
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseSince(when)
			if err != nil {
				return fmt.Errorf("parse when: %w", err)
			}
			sinceISO := t.UTC().Format("2006-01-02T15:04:05Z")

			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var entries []feedEntry

			// Calls within window.
			cRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(start_time,''), COALESCE(status,''), COALESCE(summary,'')
                 FROM calls WHERE agent_id = ? AND start_time >= ? ORDER BY start_time`,
				agentID, sinceISO,
			)
			if err != nil {
				return fmt.Errorf("query calls: %w", err)
			}
			for cRows.Next() {
				var e feedEntry
				e.Type = "call"
				if err := cRows.Scan(&e.ID, &e.Timestamp, &e.Status, &e.Summary); err != nil {
					cRows.Close()
					return fmt.Errorf("scan call: %w", err)
				}
				entries = append(entries, e)
			}
			// PATCH(greptile #560): surface mid-iteration errors instead
			// of silently truncating the feed. rows.Next() returns false
			// for both end-of-rows AND I/O / corrupted-page errors;
			// without this, callers see a partial feed as if complete.
			if err := cRows.Err(); err != nil {
				cRows.Close()
				return fmt.Errorf("iterate calls: %w", err)
			}
			cRows.Close()

			// Failing metric results within window.
			mRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(evaluated_at,''), COALESCE(metric_id,''), COALESCE(score, 0)
                 FROM metric_results
                 WHERE agent_id = ? AND evaluated_at >= ? AND (passed = 0 OR passed IS NULL AND score < 0.5)
                 ORDER BY evaluated_at`,
				agentID, sinceISO,
			)
			if err != nil {
				return fmt.Errorf("query metric_results: %w", err)
			}
			for mRows.Next() {
				var e feedEntry
				var score sql.NullFloat64
				e.Type = "metric_result"
				if err := mRows.Scan(&e.ID, &e.Timestamp, &e.MetricID, &score); err != nil {
					mRows.Close()
					return fmt.Errorf("scan metric_result: %w", err)
				}
				if score.Valid {
					e.Score = score.Float64
				}
				entries = append(entries, e)
			}
			// PATCH(greptile #560): same row-iteration error guard as
			// the calls loop above; metric_result truncation would silently
			// drop regression evidence from the audit feed.
			if err := mRows.Err(); err != nil {
				mRows.Close()
				return fmt.Errorf("iterate metric_results: %w", err)
			}
			mRows.Close()

			sort.SliceStable(entries, func(i, j int) bool { return entries[i].Timestamp < entries[j].Timestamp })
			if entries == nil {
				entries = []feedEntry{}
			}

			return flags.printJSON(cmd, map[string]any{
				"agent_id": agentID,
				"since":    sinceISO,
				"entries":  entries,
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
