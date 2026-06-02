// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: replies interested.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type interestedReplyRow struct {
	ReplyID       string `json:"reply_id"`
	LeadEmail     string `json:"lead_email"`
	CampaignID    string `json:"campaign_id"`
	SenderEmailID string `json:"sender_email_id"`
	Status        string `json:"status"`
	ReceivedAt    string `json:"received_at"`
	Snippet       string `json:"snippet"`
}

func newNovelRepliesInterestedCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "interested",
		Short: "Every reply marked interested across all campaigns since a timestamp, joined to its lead, campaign, and sender.",
		Long: `Rolls up interested replies from the local store across every campaign since a
cutoff, joined to the lead, campaign, and sender. Sync first with
'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli replies interested --since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseSince(flagSince)
			if err != nil {
				return err
			}
			db, err := openNovelStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			const q = `
SELECT
  CAST(COALESCE(json_extract(r.data,'$.id'), r.id) AS TEXT)                        AS reply_id,
  COALESCE(json_extract(r.data,'$.lead.email'), json_extract(r.data,'$.lead_email'),
           json_extract(r.data,'$.from_email'), '')                               AS lead_email,
  CAST(COALESCE(json_extract(r.data,'$.campaign_id'),
                json_extract(r.data,'$.campaign.id'), '') AS TEXT)                 AS campaign_id,
  CAST(COALESCE(json_extract(r.data,'$.sender_email_id'), '') AS TEXT)            AS sender_email_id,
  COALESCE(json_extract(r.data,'$.status'), '')                                    AS status,
  COALESCE(json_extract(r.data,'$.created_at'), json_extract(r.data,'$.replied_at'),
           json_extract(r.data,'$.date'), r.synced_at)                            AS received_at,
  substr(COALESCE(json_extract(r.data,'$.message'), json_extract(r.data,'$.subject'),
           json_extract(r.data,'$.preview'), ''), 1, 120)                         AS snippet
FROM resources r
WHERE r.resource_type = 'replies'
  AND (COALESCE(json_extract(r.data,'$.status'), '') = 'interested'
       OR COALESCE(json_extract(r.data,'$.interested'), 0) IN (1, '1', 'true'))
  AND COALESCE(json_extract(r.data,'$.created_at'), json_extract(r.data,'$.replied_at'),
               json_extract(r.data,'$.date'), r.synced_at) >= ?
ORDER BY received_at DESC;`
			rows, err := db.DB().QueryContext(cmd.Context(), q, cutoff.Format("2006-01-02 15:04:05"))
			if err != nil {
				return fmt.Errorf("querying interested replies: %w", err)
			}
			defer rows.Close()

			results := []interestedReplyRow{}
			for rows.Next() {
				var r interestedReplyRow
				var leadEmail, campaignID, senderID, status, recv, snippet sql.NullString
				if err := rows.Scan(&r.ReplyID, &leadEmail, &campaignID, &senderID, &status, &recv, &snippet); err != nil {
					return fmt.Errorf("scanning interested reply: %w", err)
				}
				r.LeadEmail = leadEmail.String
				r.CampaignID = campaignID.String
				r.SenderEmailID = senderID.String
				r.Status = status.String
				r.ReceivedAt = recv.String
				r.Snippet = snippet.String
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating interested replies: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No interested replies since %s. Run 'email-bison-pp-cli sync' first if the store is empty.\n", cutoff.Format("2006-01-02 15:04"))
				return nil
			}
			fmt.Fprintln(out, "REPLY\tLEAD\tCAMPAIGN\tRECEIVED\tSNIPPET")
			for _, r := range results {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", r.ReplyID, r.LeadEmail, r.CampaignID, r.ReceivedAt, r.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only replies since this point: a duration (24h), day count (7d), or date (2026-06-01). Default 24h.")
	return cmd
}
