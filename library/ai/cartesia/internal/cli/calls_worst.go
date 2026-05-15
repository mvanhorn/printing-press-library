// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// worstHit is the JSON shape returned per low-scored call.
type worstHit struct {
	CallID         string  `json:"id"`
	AgentID        string  `json:"agent_id,omitempty"`
	MetricID       string  `json:"metric_id,omitempty"`
	Score          float64 `json:"score"`
	Summary        string  `json:"summary,omitempty"`
	TranscriptHead string  `json:"transcript_head,omitempty"`
}

// newCallsWorstCmd registers `calls worst` — returns the N lowest-scored
// metric_results joined with their parent call rows.
func newCallsWorstCmd(flags *rootFlags) *cobra.Command {
	var metricID string
	var since string
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "worst",
		Short:       "Return the lowest-scored calls across the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  cartesia-pp-cli calls worst --limit 5 --json
  cartesia-pp-cli calls worst --metric metric_abc --since 24h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			q := `SELECT mr.call_id, COALESCE(mr.agent_id, c.agent_id, ''), COALESCE(mr.metric_id, ''),
			             COALESCE(mr.score, 0), COALESCE(c.summary, ''), COALESCE(c.transcript_text, '')
                  FROM metric_results mr
                  LEFT JOIN calls c ON c.id = mr.call_id
                  WHERE mr.score IS NOT NULL`
			var argv []any
			if metricID != "" {
				q += " AND mr.metric_id = ?"
				argv = append(argv, metricID)
			}
			if since != "" {
				t, err := parseSince(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				q += " AND mr.evaluated_at >= ?"
				argv = append(argv, t.UTC().Format("2006-01-02T15:04:05Z"))
			}
			if limit <= 0 {
				limit = 20
			}
			q += " ORDER BY mr.score ASC LIMIT ?"
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			results := []worstHit{}
			for rows.Next() {
				var h worstHit
				var transcript sql.NullString
				if err := rows.Scan(&h.CallID, &h.AgentID, &h.MetricID, &h.Score, &h.Summary, &transcript); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				if transcript.Valid {
					head := transcript.String
					if len(head) > 100 {
						head = head[:100]
					}
					h.TranscriptHead = head
				}
				results = append(results, h)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}
			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&metricID, "metric", "", "Restrict to a single metric ID")
	cmd.Flags().StringVar(&since, "since", "", "Window for evaluated_at (7d, 24h, or RFC3339)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum calls to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
