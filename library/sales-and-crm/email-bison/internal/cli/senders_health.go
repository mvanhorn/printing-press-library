// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: sender-emails health.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type senderHealthRow struct {
	SenderEmailID string `json:"sender_email_id"`
	Email         string `json:"email"`
	Type          string `json:"type"`
	State         string `json:"state"`
	LiveCampaigns int64  `json:"live_campaigns"`
	RecentBounces int64  `json:"recent_bounces"`
	Healthy       bool   `json:"healthy"`
}

func newNovelSendersHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Board joining sender state, attached campaigns, and recent bounces to spot dead or over-assigned inboxes.",
		Long: `Joins sender accounts with the campaigns they are attached to and their recent
bounced replies from the local store, so a degraded sender still attached to live
campaigns surfaces in one place. Sync first with 'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli sender-emails health --agent",
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
  CAST(COALESCE(json_extract(s.data,'$.id'), s.id) AS TEXT)                       AS sender_id,
  COALESCE(json_extract(s.data,'$.email'), json_extract(s.data,'$.email_address'), '') AS email,
  COALESCE(json_extract(s.data,'$.type'), json_extract(s.data,'$.provider'), '')  AS type,
  COALESCE(json_extract(s.data,'$.status'), json_extract(s.data,'$.state'), '')   AS state,
  (SELECT COUNT(DISTINCT a.campaigns_id) FROM attach_sender_emails a,
        json_each(COALESCE(json_extract(a.data,'$.sender_email_ids'), '[]')) j
     WHERE CAST(j.value AS TEXT) = CAST(COALESCE(json_extract(s.data,'$.id'), s.id) AS TEXT)) AS live_campaigns,
  (SELECT COUNT(*) FROM resources r
     WHERE r.resource_type = 'replies'
       AND COALESCE(json_extract(r.data,'$.folder'), '') = 'bounced'
       AND CAST(COALESCE(json_extract(r.data,'$.sender_email_id'), -1) AS TEXT)
           = CAST(COALESCE(json_extract(s.data,'$.id'), s.id) AS TEXT))           AS recent_bounces
FROM resources s
WHERE s.resource_type = 'sender-emails'
ORDER BY email;`
			rows, err := db.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return fmt.Errorf("querying sender health: %w", err)
			}
			defer rows.Close()

			results := []senderHealthRow{}
			for rows.Next() {
				var r senderHealthRow
				var email, typ, state sql.NullString
				if err := rows.Scan(&r.SenderEmailID, &email, &typ, &state, &r.LiveCampaigns, &r.RecentBounces); err != nil {
					return fmt.Errorf("scanning sender row: %w", err)
				}
				r.Email = email.String
				r.Type = typ.String
				r.State = state.String
				degraded := r.RecentBounces > 0 ||
					strings.Contains(strings.ToLower(r.State), "disconnect") ||
					strings.Contains(strings.ToLower(r.State), "error")
				r.Healthy = !degraded
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating sender rows: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No sender emails in the local store. Run 'email-bison-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintln(out, "EMAIL\tTYPE\tSTATE\tLIVE CAMPAIGNS\tRECENT BOUNCES\tHEALTHY")
			for _, r := range results {
				label := r.Email
				if label == "" {
					label = r.SenderEmailID
				}
				fmt.Fprintf(out, "%s\t%s\t%s\t%d\t%d\t%t\n", label, r.Type, r.State, r.LiveCampaigns, r.RecentBounces, r.Healthy)
			}
			return nil
		},
	}
	return cmd
}
