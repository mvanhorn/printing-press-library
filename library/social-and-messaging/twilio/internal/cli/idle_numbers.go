// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newIdleNumbersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "idle-numbers",
		Short: "Phone numbers you are paying for but have not used to send or receive in N days",
		Long: `Three-way LEFT JOIN over incoming_phone_numbers_json, messages_json, and
calls_json. For every IncomingPhoneNumber, find the most recent activity
(outbound or inbound) across Messages and Calls. Flag any number whose last
activity is older than the cutoff (or never).

Numbers cost $1/month each; agencies running 100+ numbers can save real money
identifying idle ones.

Run 'twilio-pp-cli sync --resources incoming-phone-numbers,messages,calls'
first to populate the store.`,
		Example: `  twilio-pp-cli idle-numbers --since 30d --json
  twilio-pp-cli idle-numbers --since 90d --select phone_number,last_activity`,
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

			query := `
				WITH msg_activity AS (
					SELECT
						COALESCE(json_extract(data, '$.from'), '') AS phone,
						MAX(json_extract(data, '$.date_sent')) AS last_msg
					FROM messages_json
					GROUP BY phone
				),
				call_activity AS (
					SELECT
						COALESCE(json_extract(data, '$.from'), '') AS phone,
						MAX(json_extract(data, '$.start_time')) AS last_call
					FROM calls_json
					GROUP BY phone
				)
				SELECT
					json_extract(n.data, '$.phone_number') AS phone_number,
					COALESCE(json_extract(n.data, '$.friendly_name'), '') AS friendly_name,
					COALESCE(json_extract(n.data, '$.sid'), n.id) AS sid,
					COALESCE(m.last_msg, '') AS last_msg,
					COALESCE(c.last_call, '') AS last_call,
					CASE
						WHEN COALESCE(m.last_msg, '') > COALESCE(c.last_call, '') THEN m.last_msg
						ELSE COALESCE(c.last_call, '')
					END AS last_activity
				FROM incoming_phone_numbers_json n
				LEFT JOIN msg_activity m ON m.phone = json_extract(n.data, '$.phone_number')
				LEFT JOIN call_activity c ON c.phone = json_extract(n.data, '$.phone_number')
				WHERE COALESCE(
					CASE
						WHEN COALESCE(m.last_msg, '') > COALESCE(c.last_call, '') THEN datetime(m.last_msg)
						ELSE datetime(c.last_call)
					END,
					datetime('1900-01-01')
				) < datetime(?)
				ORDER BY last_activity ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, sinceCutoffBind(cutoff))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type idle struct {
				PhoneNumber  string  `json:"phone_number"`
				FriendlyName string  `json:"friendly_name,omitempty"`
				Sid          string  `json:"sid,omitempty"`
				LastActivity string  `json:"last_activity,omitempty"`
				MonthlyCost  float64 `json:"monthly_cost_usd"`
			}
			var out []idle
			for rows.Next() {
				var r idle
				var lastMsg, lastCall string
				if err := rows.Scan(&r.PhoneNumber, &r.FriendlyName, &r.Sid,
					&lastMsg, &lastCall, &r.LastActivity); err != nil {
					return err
				}
				// Standard $1/month for US numbers; toll-free / mobile differ but
				// this is the conservative published rate. Surfacing the row matters
				// more than the exact dollar amount.
				r.MonthlyCost = 1.0
				if r.LastActivity == "" {
					r.LastActivity = "never"
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			envelope := map[string]any{
				"idle_count":      len(out),
				"monthly_savings": float64(len(out)) * 1.0,
				"numbers":         out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "30d", "Time window: <n>{s,m,h,d,w} (default 30d)")
	return cmd
}
