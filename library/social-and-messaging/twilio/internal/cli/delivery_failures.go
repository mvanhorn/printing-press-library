// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newDeliveryFailuresCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "delivery-failures",
		Short: "Failed messages grouped by error code and destination country, with cost-of-failures total",
		Long: `Local query over the synced messages_json table:

    Status IN (failed, undelivered)
    GROUPED BY (error_code, country prefix of 'to')
    SUMMING price as cost_total

The Twilio API has no aggregation by ErrorCode; the official CLI returns one
row per message. This command answers "why is delivery degraded" in one query.

Run 'twilio-pp-cli sync --resources messages' first to populate the store.`,
		Example: `  # Failures in the last 7 days, JSON for an agent
  twilio-pp-cli delivery-failures --since 7d --json

  # Last 24h, narrowed to two fields
  twilio-pp-cli delivery-failures --since 24h --select error_code,count`,
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
					COALESCE(json_extract(data, '$.error_code'), '') AS error_code,
					COALESCE(json_extract(data, '$.error_message'), '') AS error_message,
					CASE
						WHEN substr(json_extract(data, '$.to'), 1, 2) = '+1' THEN 'US/CA'
						WHEN substr(json_extract(data, '$.to'), 1, 1) = '+'
							THEN substr(json_extract(data, '$.to'), 2, 2)
						ELSE 'unknown'
					END AS country,
					COUNT(*) AS count,
					COALESCE(SUM(CAST(json_extract(data, '$.price') AS REAL)), 0) AS cost_total,
					COALESCE(json_extract(data, '$.price_unit'), '') AS price_unit
				FROM messages_json
				WHERE json_extract(data, '$.status') IN ('failed', 'undelivered')
				  AND %s >= datetime(?)
				GROUP BY error_code, country
				ORDER BY count DESC
				LIMIT 200`, twilioDateExpr("date_sent"))

			rows, err := db.DB().QueryContext(cmd.Context(), query, sinceCutoffBind(cutoff))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type result struct {
				ErrorCode    string  `json:"error_code"`
				ErrorMessage string  `json:"error_message,omitempty"`
				ToCountry    string  `json:"to_country"`
				Count        int     `json:"count"`
				CostTotal    float64 `json:"cost_total"`
				PriceUnit    string  `json:"price_unit,omitempty"`
			}
			var out []result
			for rows.Next() {
				var r result
				if err := rows.Scan(&r.ErrorCode, &r.ErrorMessage, &r.ToCountry,
					&r.Count, &r.CostTotal, &r.PriceUnit); err != nil {
					return err
				}
				// Take the absolute value: Twilio prices are returned as negative numbers
				// (debits). Display as positive cost-of-failure for human readability.
				if r.CostTotal < 0 {
					r.CostTotal = -r.CostTotal
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: <n>{s,m,h,d,w} (e.g. 24h, 7d, 4w)")
	return cmd
}
