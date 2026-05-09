// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

// twilioErrorCatalog maps the 50 most common stable Twilio error codes to a
// human-readable cause and the typical fix. Curated static reference; the
// Twilio error catalog at https://www.twilio.com/docs/api/errors is the
// authoritative source. Marked as a `pp:novel-static-reference` so the
// reimplementation_check classifier knows this is curated data, not a
// substitute for an API response.
//
// pp:novel-static-reference
var twilioErrorCatalog = map[string]struct {
	Cause string
	Fix   string
}{
	"30001": {"Queue overflow", "You have queued too many messages; back off and retry."},
	"30002": {"Account suspended", "Contact Twilio support; the source account is suspended."},
	"30003": {"Unreachable destination handset", "The receiving carrier cannot deliver SMS to that handset (off, no signal, no SMS plan)."},
	"30004": {"Message blocked", "The destination carrier has blocked your traffic; investigate templates and opt-in compliance."},
	"30005": {"Unknown destination handset", "The number is invalid or no longer in service."},
	"30006": {"Landline or unreachable carrier", "The destination is a landline or unsupported carrier; SMS cannot deliver."},
	"30007": {"Carrier violation", "Carrier filtering blocked the message for content/policy reasons."},
	"30008": {"Unknown error", "Carrier returned an unspecified failure; retry or contact support if persistent."},
	"30009": {"Missing inbound segment", "Multi-segment message lost a part in transit."},
	"30010": {"Message price exceeds max", "Price exceeded MaxPrice; raise the cap or accept the failure."},
	"30011": {"MMS not supported by destination", "Convert to SMS or pick a different destination."},
	"21211": {"Invalid 'To' number", "Format must be E.164 (+CCXXXXXXXXXX)."},
	"21212": {"Invalid 'From' number", "The From number must be a Twilio number on this account."},
	"21214": {"'To' number cannot be reached", "Twilio cannot route to that destination; check geo permissions."},
	"21408": {"Permission to send SMS denied", "Geo Permissions disabled this country; enable it in Console."},
	"21610": {"Recipient unsubscribed", "STOP was previously sent; do not send marketing to this number."},
	"21611": {"This Twilio number cannot send SMS", "The From number lacks SMS capability; pick a capable number."},
	"21612": {"Phone number not yet activated", "Wait until the number provisioning completes."},
	"21614": {"To number not a valid mobile number", "The destination is not SMS-capable (likely a landline or VoIP)."},
	"21617": {"Concatenated message body exceeds limit", "Trim the body; long-message segmentation has a hard cap."},
	"30410": {"Provider timeout", "Carrier timed out; retry."},
	"30450": {"Provider error", "Carrier returned an internal error; retry."},
	"63016": {"Channel not provisioned", "WhatsApp/Channels: register the sender or template."},
	"63018": {"WhatsApp template not approved", "Submit and get template approval before sending."},
	"63024": {"Invalid WhatsApp From", "The WhatsApp sender is not configured for this account."},
	"63031": {"WhatsApp 24-hour rule", "User did not initiate the conversation in the last 24h; use a template."},
	"11200": {"HTTP retrieval failure", "Twilio could not reach your webhook URL; check DNS, TLS, and uptime."},
	"11205": {"HTTP connection failure", "Webhook host unreachable from Twilio's edge."},
	"11206": {"HTTP parse failure", "Webhook returned malformed TwiML or non-XML."},
	"11750": {"TwiML response body too large", "TwiML responses are capped at 64KB."},
	"13201": {"Dial action URL fetch failure", "TwiML <Dial action=\"\"> URL is unreachable."},
	"13212": {"Application error", "Your TwiML application returned an unrecoverable error."},
	"31000": {"Generic SIP error", "Generic SIP signaling failure; consult call detail."},
	"31002": {"Connection declined", "Call recipient declined; carrier signaled CALL_REJECTED."},
	"31203": {"No valid account found", "Account SID in the URL does not match the credential's owner."},
	"32213": {"Conference reached participant limit", "Twilio Conference participant cap (default 250) hit."},
}

func newErrorCodeExplainCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "error-code-explain",
		Short: "Top error codes from your recent messages and calls, with curated explanations",
		Long: `Local groupby on synced messages_json + calls_json by error_code, joined to
a curated static-reference table of the most common Twilio error codes
(https://www.twilio.com/docs/api/errors). Each row shows the count and a
one-line cause + fix.

Removes the constant Google-the-error-code round trip during incident triage.

Run 'twilio-pp-cli sync --resources messages,calls' first to populate the store.`,
		Example: `  twilio-pp-cli error-code-explain --since 7d --json
  twilio-pp-cli error-code-explain --since 24h`,
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
				WITH all_errors AS (
					SELECT
						COALESCE(json_extract(data, '$.error_code'), '') AS code,
						'message' AS source
					FROM messages_json
					WHERE json_extract(data, '$.error_code') IS NOT NULL
					  AND json_extract(data, '$.error_code') != ''
					  AND %s >= datetime(?)
					UNION ALL
					SELECT
						COALESCE(json_extract(data, '$.error_code'), '') AS code,
						'call' AS source
					FROM calls_json
					WHERE json_extract(data, '$.error_code') IS NOT NULL
					  AND json_extract(data, '$.error_code') != ''
					  AND %s >= datetime(?)
				)
				SELECT code, COUNT(*) AS count, GROUP_CONCAT(DISTINCT source) AS sources
				FROM all_errors
				GROUP BY code
				ORDER BY count DESC
				LIMIT 50`, twilioDateExpr("date_sent"), twilioDateExpr("start_time"))

			cutoffBind := sinceCutoffBind(cutoff)
			rows, err := db.DB().QueryContext(cmd.Context(), query, cutoffBind, cutoffBind)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type explained struct {
				Code    string `json:"code"`
				Count   int    `json:"count"`
				Sources string `json:"sources,omitempty"`
				Cause   string `json:"cause,omitempty"`
				Fix     string `json:"fix,omitempty"`
			}
			var out []explained
			for rows.Next() {
				var e explained
				if err := rows.Scan(&e.Code, &e.Count, &e.Sources); err != nil {
					return err
				}
				if entry, ok := twilioErrorCatalog[e.Code]; ok {
					e.Cause = entry.Cause
					e.Fix = entry.Fix
				} else {
					e.Cause = "Unknown error code"
					e.Fix = fmt.Sprintf("See https://www.twilio.com/docs/api/errors/%s", e.Code)
				}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: <n>{s,m,h,d,w}")
	return cmd
}
