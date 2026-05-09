// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newCallDispositionCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "call-disposition",
		Short: "Cross-tab of call Status (completed, busy, no-answer, failed, canceled) by AnsweredBy with $ cost per bucket",
		Long: `Local query over the synced calls_json table. Cross-tabs the call's
terminal Status against AnsweredBy (human, machine_start, machine_end_beep,
fax) and sums the price per bucket.

Voicemail-detection rates are a key Twilio Voice metric; no CLI exposes them.

Run 'twilio-pp-cli sync --resources calls' first to populate the store.`,
		Example: `  twilio-pp-cli call-disposition --since 7d --json
  twilio-pp-cli call-disposition --since 24h --select status,answered_by,count`,
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

			query := fmt.Sprintf(`
				SELECT
					COALESCE(json_extract(data, '$.status'), 'unknown') AS status,
					COALESCE(json_extract(data, '$.answered_by'), 'unknown') AS answered_by,
					COUNT(*) AS count,
					COALESCE(SUM(CAST(json_extract(data, '$.price') AS REAL)), 0) AS cost_total,
					COALESCE(AVG(CAST(json_extract(data, '$.duration') AS INTEGER)), 0) AS avg_duration_seconds
				FROM calls_json
				WHERE %s >= datetime(?)
				GROUP BY status, answered_by
				ORDER BY count DESC
				LIMIT 200`, twilioDateExpr("start_time"))

			rows, err := db.DB().QueryContext(cmd.Context(), query, sinceCutoffBind(cutoff))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Status      string  `json:"status"`
				AnsweredBy  string  `json:"answered_by"`
				Count       int     `json:"count"`
				CostTotal   float64 `json:"cost_total"`
				AvgDuration float64 `json:"avg_duration_seconds"`
			}
			var out []bucket
			for rows.Next() {
				var b bucket
				if err := rows.Scan(&b.Status, &b.AnsweredBy, &b.Count,
					&b.CostTotal, &b.AvgDuration); err != nil {
					return err
				}
				if b.CostTotal < 0 {
					b.CostTotal = -b.CostTotal
				}
				out = append(out, b)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "24h", "Time window: <n>{s,m,h,d,w}")
	return cmd
}
