// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newConversationCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "conversation [phone-number]",
		Short: "All messages and calls involving one phone number, merged into a single timestamped timeline",
		Long: `UNION over synced messages_json and calls_json. For the given phone number,
merge every Message and every Call where From=<num> OR To=<num> into a single
chronological timeline with direction (in/out), body or duration, and status.

Twilio Console has a per-number log; no other CLI exposes the merged view.

Run 'twilio-pp-cli sync --resources messages,calls' first to populate the store.`,
		Example: `  twilio-pp-cli conversation +14155551234 --json
  twilio-pp-cli conversation +14155551234 --limit 50`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			number := strings.TrimSpace(args[0])
			if number == "" {
				return usageErr(fmt.Errorf("phone number is required (e.g. +14155551234)"))
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
				SELECT
					'message' AS kind,
					json_extract(data, '$.sid') AS sid,
					json_extract(data, '$.date_sent') AS ts,
					json_extract(data, '$.direction') AS direction,
					json_extract(data, '$.from') AS from_num,
					json_extract(data, '$.to') AS to_num,
					COALESCE(json_extract(data, '$.body'), '') AS body_or_duration,
					COALESCE(json_extract(data, '$.status'), '') AS status,
					COALESCE(CAST(json_extract(data, '$.price') AS REAL), 0) AS price
				FROM messages_json
				WHERE json_extract(data, '$.from') = ? OR json_extract(data, '$.to') = ?
				UNION ALL
				SELECT
					'call' AS kind,
					json_extract(data, '$.sid') AS sid,
					json_extract(data, '$.start_time') AS ts,
					json_extract(data, '$.direction') AS direction,
					json_extract(data, '$.from') AS from_num,
					json_extract(data, '$.to') AS to_num,
					COALESCE(json_extract(data, '$.duration'), '') AS body_or_duration,
					COALESCE(json_extract(data, '$.status'), '') AS status,
					COALESCE(CAST(json_extract(data, '$.price') AS REAL), 0) AS price
				FROM calls_json
				WHERE json_extract(data, '$.from') = ? OR json_extract(data, '$.to') = ?
				ORDER BY ts DESC
				LIMIT ?`

			rows, err := db.DB().QueryContext(cmd.Context(), query,
				number, number, number, number, limit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type entry struct {
				Kind      string  `json:"kind"`
				Sid       string  `json:"sid"`
				Timestamp string  `json:"timestamp"`
				Direction string  `json:"direction,omitempty"`
				From      string  `json:"from"`
				To        string  `json:"to"`
				Body      string  `json:"body,omitempty"`
				Duration  string  `json:"duration_seconds,omitempty"`
				Status    string  `json:"status,omitempty"`
				Price     float64 `json:"price,omitempty"`
			}
			var out []entry
			for rows.Next() {
				var kind, sid, ts, dir, fromN, toN, bod, status string
				var price float64
				if err := rows.Scan(&kind, &sid, &ts, &dir, &fromN, &toN, &bod, &status, &price); err != nil {
					return err
				}
				e := entry{Kind: kind, Sid: sid, Timestamp: ts, Direction: dir,
					From: fromN, To: toN, Status: status, Price: price}
				if kind == "message" {
					e.Body = bod
				} else {
					e.Duration = bod
				}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			envelope := map[string]any{
				"phone_number": number,
				"count":        len(out),
				"entries":      out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum entries to return")
	return cmd
}
