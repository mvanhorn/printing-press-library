// Copyright 2026 bossriceshark. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// campaignHealth is one row of the health scorecard. Rates are fractions
// (0.0–1.0) relative to the count of leads that were actually sent to.
type campaignHealth struct {
	CampaignID   string  `json:"campaign_id"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	Leads        int     `json:"leads"`
	SentLeads    int     `json:"sent_leads"`
	OpenRate     float64 `json:"open_rate"`
	ReplyRate    float64 `json:"reply_rate"`
	BounceRate   float64 `json:"bounce_rate"`
	SilentLeads  int     `json:"silent_leads"`
	LastActivity string  `json:"last_activity,omitempty"`
	Stale        bool    `json:"stale"`
}

// leadAgg accumulates per-lead send outcomes within one campaign.
type leadAgg struct {
	sent, opened, replied, bounced bool
	lastSent                       string
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath, campaignID string
	var silentDays, staleDays int

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Campaign health scorecard from the local mirror",
		Long: strings.Trim(`
Compute a one-shot health scorecard for every synced campaign — open rate,
reply rate, bounce rate, silent-lead count, and a stale flag — by joining the
local campaigns, statistics, and leads tables. The SmartLead API has no
campaign-health endpoint; answering this live costs four-plus calls per
campaign. Run 'smartlead-pp-cli sync' first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli health --json
  smartlead-pp-cli health --campaign 3344703
  smartlead-pp-cli health --silent-days 10 --stale-days 21`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openMirror(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			campaigns, err := loadCampaignMeta(cmd.Context(), db.DB(), campaignID)
			if err != nil {
				return err
			}
			if len(campaigns) == 0 {
				return emitEmpty[campaignHealth](cmd, flags, "campaigns")
			}

			leadCounts := map[string]int{}
			if rows, qerr := db.DB().QueryContext(cmd.Context(),
				`SELECT campaigns_id, COUNT(*) FROM campaigns_leads GROUP BY campaigns_id`); qerr == nil {
				for rows.Next() {
					var cid string
					var n int
					if rows.Scan(&cid, &n) == nil {
						leadCounts[cid] = n
					}
				}
				rows.Close()
				if rerr := rows.Err(); rerr != nil {
					fmt.Fprintf(os.Stderr, "warning: campaign lead counts incomplete: %v\n", rerr)
				}
			} else {
				fmt.Fprintf(os.Stderr, "warning: campaign lead counts unavailable (%v); leads will show 0 — run 'sync' first\n", qerr)
			}

			// Per-campaign, per-lead aggregation from the statistics table.
			perCampaign := map[string]map[string]*leadAgg{}
			rows, qerr := db.DB().QueryContext(cmd.Context(), `SELECT campaigns_id,
				json_extract(data,'$.lead_email'), json_extract(data,'$.sent_time'),
				json_extract(data,'$.open_time'), json_extract(data,'$.reply_time'),
				json_extract(data,'$.is_bounced') FROM statistics`)
			if qerr != nil {
				return apiErr(fmt.Errorf("reading statistics: %w", qerr))
			}
			for rows.Next() {
				var cid string
				var email, sentT, openT, replyT, bounced sql.NullString
				if rows.Scan(&cid, &email, &sentT, &openT, &replyT, &bounced) != nil {
					continue
				}
				if !email.Valid || email.String == "" {
					continue
				}
				byLead := perCampaign[cid]
				if byLead == nil {
					byLead = map[string]*leadAgg{}
					perCampaign[cid] = byLead
				}
				key := strings.ToLower(email.String)
				agg := byLead[key]
				if agg == nil {
					agg = &leadAgg{}
					byLead[key] = agg
				}
				if sentT.Valid && sentT.String != "" {
					agg.sent = true
					if sentT.String > agg.lastSent {
						agg.lastSent = sentT.String
					}
				}
				if openT.Valid && openT.String != "" {
					agg.opened = true
				}
				if replyT.Valid && replyT.String != "" {
					agg.replied = true
				}
				if bounced.Valid && truthy(bounced.String) {
					agg.bounced = true
				}
			}
			rows.Close()
			if rerr := rows.Err(); rerr != nil {
				return apiErr(fmt.Errorf("reading statistics: %w", rerr))
			}

			now := nowUTC()
			var out []campaignHealth
			for cid, meta := range campaigns {
				h := campaignHealth{
					CampaignID: cid,
					Name:       meta.name,
					Status:     meta.status,
					Leads:      leadCounts[cid],
				}
				var sent, opened, replied, bounced, silent int
				var lastActivity string
				for _, agg := range perCampaign[cid] {
					if !agg.sent {
						continue
					}
					sent++
					if agg.opened {
						opened++
					}
					if agg.replied {
						replied++
					}
					if agg.bounced {
						bounced++
					}
					if agg.lastSent > lastActivity {
						lastActivity = agg.lastSent
					}
					if !agg.replied {
						if t, ok := parseSLTime(agg.lastSent); ok &&
							now.Sub(t).Hours() >= float64(silentDays*24) {
							silent++
						}
					}
				}
				h.SentLeads = sent
				h.OpenRate = rate(opened, sent)
				h.ReplyRate = rate(replied, sent)
				h.BounceRate = rate(bounced, sent)
				h.SilentLeads = silent
				h.LastActivity = lastActivity
				if t, ok := parseSLTime(lastActivity); ok {
					h.Stale = now.Sub(t).Hours() >= float64(staleDays*24)
				} else {
					h.Stale = sent > 0
				}
				out = append(out, h)
			}

			// Worst-first: campaigns with sends ranked by reply rate ascending;
			// campaigns with zero sends sink to the bottom.
			sort.Slice(out, func(i, j int) bool {
				a, b := out[i], out[j]
				if (a.SentLeads == 0) != (b.SentLeads == 0) {
					return b.SentLeads == 0
				}
				if a.ReplyRate != b.ReplyRate {
					return a.ReplyRate < b.ReplyRate
				}
				return a.Name < b.Name
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CAMPAIGN\tSTATUS\tLEADS\tSENT\tOPEN%\tREPLY%\tBOUNCE%\tSILENT\tSTALE")
			for _, h := range out {
				stale := ""
				if h.Stale {
					stale = "stale"
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%.1f\t%.1f\t%.1f\t%d\t%s\n",
					truncate(h.Name, 36), h.Status, h.Leads, h.SentLeads,
					h.OpenRate*100, h.ReplyRate*100, h.BounceRate*100, h.SilentLeads, stale)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/smartlead-pp-cli/data.db)")
	cmd.Flags().StringVar(&campaignID, "campaign", "", "Limit the scorecard to one campaign ID")
	cmd.Flags().IntVar(&silentDays, "silent-days", 7, "A sent lead with no reply for this many days counts as silent")
	cmd.Flags().IntVar(&staleDays, "stale-days", 14, "A campaign with no send activity for this many days is flagged stale")
	return cmd
}
