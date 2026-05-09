// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newOptOutViolationsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "opt-out-violations",
		Short: "Find numbers that texted STOP/UNSUBSCRIBE/END/QUIT and any messages your account sent to them afterwards",
		Long: `Local temporal join over messages_json. For each inbound Message whose
trimmed body matches a TCPA opt-out keyword (STOP, UNSUBSCRIBE, END, QUIT,
CANCEL, REVOKE), find any subsequent outbound Message to that same phone
number with date_sent later than the opt-out.

Twilio has no opt-out resource. TCPA fines are $500-$1,500 per violation, so
this is the audit compliance teams pay legal teams to construct manually.

Run 'twilio-pp-cli sync --resources messages' first to populate the store.`,
		Example: `  twilio-pp-cli opt-out-violations --since 90d --json
  twilio-pp-cli opt-out-violations --since 30d --csv > violations.csv`,
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

			// Match the opt-out keywords case-insensitively. SMS bodies may carry
			// trailing whitespace or punctuation; we trim and lowercase before
			// comparing.
			query := fmt.Sprintf(`
				WITH opt_outs AS (
					SELECT
						json_extract(data, '$.from') AS subscriber,
						json_extract(data, '$.to') AS our_number,
						json_extract(data, '$.date_sent') AS opted_out_at,
						UPPER(TRIM(REPLACE(REPLACE(REPLACE(json_extract(data, '$.body'), '.', ''), '!', ''), '?', ''))) AS keyword
					FROM messages_json
					WHERE json_extract(data, '$.direction') = 'inbound'
					  AND UPPER(TRIM(REPLACE(REPLACE(REPLACE(json_extract(data, '$.body'), '.', ''), '!', ''), '?', '')))
					      IN ('STOP', 'UNSUBSCRIBE', 'END', 'QUIT', 'CANCEL', 'REVOKE')
					  AND %s >= datetime(?)
				),
				violations AS (
					SELECT
						o.subscriber,
						o.our_number,
						o.opted_out_at,
						o.keyword,
						json_extract(m.data, '$.sid') AS subsequent_sid,
						json_extract(m.data, '$.date_sent') AS subsequent_sent_at,
						json_extract(m.data, '$.body') AS subsequent_body
					FROM opt_outs o
					JOIN messages_json m
						ON json_extract(m.data, '$.to') = o.subscriber
						AND json_extract(m.data, '$.direction') LIKE 'outbound%%'
					WHERE datetime(json_extract(m.data, '$.date_sent')) > datetime(o.opted_out_at)
				)
				SELECT subscriber, our_number, opted_out_at, keyword,
				       subsequent_sid, subsequent_sent_at, subsequent_body
				FROM violations
				ORDER BY opted_out_at DESC
				LIMIT 500`, twilioDateExpr("date_sent"))

			rows, err := db.DB().QueryContext(cmd.Context(), query, sinceCutoffBind(cutoff))
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type violation struct {
				Subscriber       string `json:"subscriber"`
				OurNumber        string `json:"our_number"`
				OptedOutAt       string `json:"opted_out_at"`
				Keyword          string `json:"keyword"`
				SubsequentSid    string `json:"subsequent_sid"`
				SubsequentSentAt string `json:"subsequent_sent_at"`
				SubsequentBody   string `json:"subsequent_body,omitempty"`
			}
			var out []violation
			for rows.Next() {
				var v violation
				if err := rows.Scan(&v.Subscriber, &v.OurNumber, &v.OptedOutAt,
					&v.Keyword, &v.SubsequentSid, &v.SubsequentSentAt, &v.SubsequentBody); err != nil {
					return err
				}
				out = append(out, v)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			envelope := map[string]any{
				"violation_count": len(out),
				"violations":      out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "90d", "Audit window: <n>{s,m,h,d,w} (default 90d)")
	return cmd
}
