// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: campaigns headroom.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type headroomRow struct {
	CampaignID string `json:"campaign_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	MaxPerDay  int64  `json:"max_emails_per_day"`
	SentToday  int64  `json:"sent_today"`
	Headroom   int64  `json:"headroom"`
	State      string `json:"state"`
}

func newNovelCampaignsHeadroomCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headroom",
		Short: "See which launched campaigns are sending below their daily cap, at cap, or idle, in one table.",
		Long: `Joins each campaign's daily-cap setting against today's sent-email counts in the
local store to show which launched campaigns are sending below cap, at cap, or
idle. Sync first with 'email-bison-pp-cli sync'. No single API call answers this.`,
		Example:     "  email-bison-pp-cli campaigns headroom --agent",
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

			// Campaign cap from the campaign row; today's sent count from the
			// sent_emails child table joined by campaign_id in its JSON payload.
			const q = `
SELECT
  COALESCE(json_extract(c.data,'$.id'), c.id)                                   AS campaign_id,
  COALESCE(json_extract(c.data,'$.name'), '')                                   AS name,
  COALESCE(json_extract(c.data,'$.status'), json_extract(c.data,'$.state'), '') AS status,
  COALESCE(json_extract(c.data,'$.max_emails_per_day'),
           json_extract(c.data,'$.settings.max_emails_per_day'), 0)             AS max_per_day,
  (SELECT COUNT(*) FROM sent_emails se
     WHERE CAST(COALESCE(json_extract(se.data,'$.campaign_id'),
                         json_extract(se.data,'$.campaign.id'), -1) AS TEXT)
           = CAST(COALESCE(json_extract(c.data,'$.id'), c.id) AS TEXT)
       AND date(COALESCE(json_extract(se.data,'$.sent_at'),
                         json_extract(se.data,'$.created_at'), se.synced_at))
           = date('now'))                                                       AS sent_today
FROM resources c
WHERE c.resource_type = 'campaigns'
ORDER BY name;`
			rows, err := db.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return fmt.Errorf("querying campaign headroom: %w", err)
			}
			defer rows.Close()

			results := []headroomRow{}
			for rows.Next() {
				var r headroomRow
				var name, status sql.NullString
				if err := rows.Scan(&r.CampaignID, &name, &status, &r.MaxPerDay, &r.SentToday); err != nil {
					return fmt.Errorf("scanning headroom row: %w", err)
				}
				r.Name = name.String
				r.Status = status.String
				r.Headroom = r.MaxPerDay - r.SentToday
				switch {
				case r.SentToday == 0:
					r.State = "idle"
				case r.MaxPerDay > 0 && r.SentToday >= r.MaxPerDay:
					r.State = "at_cap"
				default:
					r.State = "under_cap"
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating headroom rows: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No campaigns in the local store. Run 'email-bison-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintln(out, "CAMPAIGN\tSTATUS\tCAP/DAY\tSENT TODAY\tHEADROOM\tSTATE")
			for _, r := range results {
				label := r.Name
				if label == "" {
					label = r.CampaignID
				}
				fmt.Fprintf(out, "%s\t%s\t%d\t%d\t%d\t%s\n", label, r.Status, r.MaxPerDay, r.SentToday, r.Headroom, r.State)
			}
			return nil
		},
	}
	return cmd
}
