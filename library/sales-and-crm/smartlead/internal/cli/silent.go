// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// silentLead is one lead that was emailed but has not replied within the
// silence window.
type silentLead struct {
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name,omitempty"`
	LeadEmail    string `json:"lead_email"`
	LeadName     string `json:"lead_name,omitempty"`
	LastSent     string `json:"last_sent"`
	DaysSilent   int    `json:"days_silent"`
}

// silentAgg tracks one lead's send/reply state while scanning statistics.
type silentAgg struct {
	sent, replied bool
	lastSent      string
	name          string
}

func newSilentCmd(flags *rootFlags) *cobra.Command {
	var dbPath, campaignID string
	var days int

	cmd := &cobra.Command{
		Use:   "silent",
		Short: "Leads emailed but silent for N days, from the local mirror",
		Long: strings.Trim(`
Find leads that were sent at least one email but have not replied within the
silence window — the exact set to follow up with or retire. Computed from the
local statistics mirror by diffing per-lead send and reply timestamps; no
SmartLead endpoint answers "sent but no reply in N days". Run
'smartlead-pp-cli sync' first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli silent --campaign 3344703 --days 7
  smartlead-pp-cli silent --days 14 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be at least 1"))
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
				return emitEmpty[silentLead](cmd, flags, "campaigns")
			}

			q := `SELECT campaigns_id, json_extract(data,'$.lead_email'),
				json_extract(data,'$.lead_name'), json_extract(data,'$.sent_time'),
				json_extract(data,'$.reply_time') FROM statistics`
			var qargs []any
			if campaignID != "" {
				q += " WHERE campaigns_id = ?"
				qargs = append(qargs, campaignID)
			}
			rows, qerr := db.DB().QueryContext(cmd.Context(), q, qargs...)
			if qerr != nil {
				return apiErr(fmt.Errorf("reading statistics: %w", qerr))
			}
			perCampaign := map[string]map[string]*silentAgg{}
			for rows.Next() {
				var cid string
				var email, name, sentT, replyT sql.NullString
				if rows.Scan(&cid, &email, &name, &sentT, &replyT) != nil {
					continue
				}
				if !email.Valid || email.String == "" {
					continue
				}
				byLead := perCampaign[cid]
				if byLead == nil {
					byLead = map[string]*silentAgg{}
					perCampaign[cid] = byLead
				}
				key := strings.ToLower(email.String)
				agg := byLead[key]
				if agg == nil {
					agg = &silentAgg{name: strings.TrimSpace(name.String)}
					byLead[key] = agg
				}
				if sentT.Valid && sentT.String != "" {
					agg.sent = true
					if sentT.String > agg.lastSent {
						agg.lastSent = sentT.String
					}
				}
				if replyT.Valid && replyT.String != "" {
					agg.replied = true
				}
			}
			rows.Close()
			if rerr := rows.Err(); rerr != nil {
				return apiErr(fmt.Errorf("reading statistics: %w", rerr))
			}

			now := nowUTC()
			var out []silentLead
			for cid, byLead := range perCampaign {
				for email, agg := range byLead {
					if !agg.sent || agg.replied {
						continue
					}
					t, ok := parseSLTime(agg.lastSent)
					if !ok {
						continue
					}
					daysSilent := int(now.Sub(t).Hours() / 24)
					if daysSilent < days {
						continue
					}
					out = append(out, silentLead{
						CampaignID:   cid,
						CampaignName: campaigns[cid].name,
						LeadEmail:    email,
						LeadName:     agg.name,
						LastSent:     agg.lastSent,
						DaysSilent:   daysSilent,
					})
				}
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].DaysSilent != out[j].DaysSilent {
					return out[i].DaysSilent > out[j].DaysSilent
				}
				return out[i].LeadEmail < out[j].LeadEmail
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No silent leads in the window.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DAYS\tLEAD\tCAMPAIGN\tLAST SENT")
			for _, s := range out {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
					s.DaysSilent, truncate(s.LeadEmail, 40), truncate(s.CampaignName, 32), s.LastSent)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/smartlead-pp-cli/data.db)")
	cmd.Flags().StringVar(&campaignID, "campaign", "", "Limit to one campaign ID (default: all synced campaigns)")
	cmd.Flags().IntVar(&days, "days", 7, "Minimum days since the last send with no reply")
	return cmd
}
