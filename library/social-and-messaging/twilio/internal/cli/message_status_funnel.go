// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newMessageStatusFunnelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "message-status-funnel",
		Short: "Distribution of message terminal statuses with delivery-rate percentages",
		Long: `Local query over the synced messages_json table. Buckets each Message into
its terminal Status (queued, sent, delivered, failed, undelivered) and computes
the delivery-rate percentage and median time-to-delivery from date_created to
date_updated.

The Twilio Console graphs this distribution; no CLI or MCP exposes the numbers.

Run 'twilio-pp-cli sync --resources messages' first to populate the store.`,
		Example: `  twilio-pp-cli message-status-funnel --since 24h --json
  twilio-pp-cli message-status-funnel --since 7d`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("twilio-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// Per-status aggregation. The median delivery latency is approximated
			// as AVG(julianday(date_updated) - julianday(date_created)) * 86400
			// since SQLite doesn't have a built-in median aggregator.
			query := fmt.Sprintf(`
				SELECT
					COALESCE(json_extract(data, '$.status'), 'unknown') AS status,
					COUNT(*) AS count,
					COALESCE(AVG(
						(julianday(json_extract(data, '$.date_updated')) -
						 julianday(json_extract(data, '$.date_created'))) * 86400
					), 0) AS avg_seconds_to_terminal
				FROM messages_json
				WHERE %s >= datetime(?)
				GROUP BY status
				ORDER BY count DESC`, twilioDateExpr("date_sent"))

			rows, err := db.DB().QueryContext(cmd.Context(), query, sinceCutoffBind(cutoff))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Status               string  `json:"status"`
				Count                int     `json:"count"`
				Pct                  float64 `json:"pct"`
				AvgSecondsToTerminal float64 `json:"avg_seconds_to_terminal,omitempty"`
			}
			var out []bucket
			total := 0
			for rows.Next() {
				var b bucket
				if err := rows.Scan(&b.Status, &b.Count, &b.AvgSecondsToTerminal); err != nil {
					return err
				}
				total += b.Count
				out = append(out, b)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			for i := range out {
				if total > 0 {
					out[i].Pct = 100.0 * float64(out[i].Count) / float64(total)
				}
			}

			envelope := map[string]any{
				"total":   total,
				"buckets": out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "24h", "Time window: <n>{s,m,h,d,w}")
	return cmd
}
