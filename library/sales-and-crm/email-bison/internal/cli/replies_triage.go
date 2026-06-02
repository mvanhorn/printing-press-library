// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: replies triage.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type triageRow struct {
	ReplyID         string `json:"reply_id"`
	LeadEmail       string `json:"lead_email"`
	CampaignID      string `json:"campaign_id"`
	ReceivedAt      string `json:"received_at"`
	Snippet         string `json:"snippet"`
	SuggestedAction string `json:"suggested_action"`
}

func newNovelRepliesTriageCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "An oldest-first worklist of pending replies with lead and campaign context, ready to act on.",
		Long: `Builds an oldest-first worklist of inbox replies that have not been categorized
yet, with lead and campaign context. Pipe reply IDs into 'replies mark-as-interested'
or 'replies followup-campaign push'. Sync first with 'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli replies triage --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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
  COALESCE(json_extract(r.data,'$.created_at'), json_extract(r.data,'$.replied_at'),
           json_extract(r.data,'$.date'), r.synced_at)                            AS received_at,
  substr(COALESCE(json_extract(r.data,'$.message'), json_extract(r.data,'$.subject'),
           json_extract(r.data,'$.preview'), ''), 1, 120)                         AS snippet
FROM resources r
WHERE r.resource_type = 'replies'
  AND COALESCE(json_extract(r.data,'$.folder'), 'inbox') = 'inbox'
  AND COALESCE(json_extract(r.data,'$.status'), '') NOT IN ('interested')
  AND COALESCE(json_extract(r.data,'$.read'), 0) IN (0, '0', 'false')
ORDER BY received_at ASC
LIMIT ?;`
			rows, err := db.DB().QueryContext(cmd.Context(), q, flagLimit)
			if err != nil {
				return fmt.Errorf("querying triage queue: %w", err)
			}
			defer rows.Close()

			results := []triageRow{}
			for rows.Next() {
				var r triageRow
				var leadEmail, campaignID, recv, snippet sql.NullString
				if err := rows.Scan(&r.ReplyID, &leadEmail, &campaignID, &recv, &snippet); err != nil {
					return fmt.Errorf("scanning triage row: %w", err)
				}
				r.LeadEmail = leadEmail.String
				r.CampaignID = campaignID.String
				r.ReceivedAt = recv.String
				r.Snippet = snippet.String
				r.SuggestedAction = "review: mark-as-interested or followup-campaign push"
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating triage rows: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "Triage queue empty. Run 'email-bison-pp-cli sync' first if the store is empty.")
				return nil
			}
			fmt.Fprintln(out, "REPLY\tLEAD\tCAMPAIGN\tRECEIVED\tSNIPPET")
			for _, r := range results {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", r.ReplyID, r.LeadEmail, r.CampaignID, r.ReceivedAt, r.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum replies to include in the queue.")
	return cmd
}
