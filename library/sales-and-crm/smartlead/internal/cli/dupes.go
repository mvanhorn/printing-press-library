// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// leadMembership is one campaign a lead belongs to.
type leadMembership struct {
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name,omitempty"`
	Status       string `json:"status,omitempty"`
	AddedAt      string `json:"added_at,omitempty"`
}

// dupeEntry is one lead and every campaign it appears in.
type dupeEntry struct {
	LeadEmail     string           `json:"lead_email"`
	Domain        string           `json:"domain"`
	CampaignCount int              `json:"campaign_count"`
	Campaigns     []leadMembership `json:"campaigns"`
}

func newDupesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, email, domain string

	cmd := &cobra.Command{
		Use:   "dupes",
		Short: "Find leads or domains contacted across multiple campaigns",
		Long: strings.Trim(`
Scan the local leads mirror for cross-campaign collisions. With no flags it
lists every lead present in two or more campaigns — the double-contact guard
to run before adding a new batch. With --email it shows one lead's full
campaign membership; with --domain it prints the pitch ledger for a site
(every lead at that domain and the campaigns they touch). The SmartLead API
is campaign-scoped and cannot answer this in one call. Run
'smartlead-pp-cli sync' first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli dupes --json
  smartlead-pp-cli dupes --domain example.com
  smartlead-pp-cli dupes --email lead@example.com`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if email != "" && domain != "" {
				return usageErr(fmt.Errorf("pass --email or --domain, not both"))
			}
			db, err := openMirror(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			campaigns, err := loadCampaignMeta(cmd.Context(), db.DB(), "")
			if err != nil {
				return err
			}

			rows, qerr := db.DB().QueryContext(cmd.Context(), `SELECT campaigns_id,
				json_extract(data,'$.lead.email'), json_extract(data,'$.status'),
				json_extract(data,'$.created_at') FROM campaigns_leads`)
			if qerr != nil {
				return apiErr(fmt.Errorf("reading leads: %w", qerr))
			}
			// email -> campaign_id -> membership (deduped per campaign)
			byLead := map[string]map[string]leadMembership{}
			for rows.Next() {
				var cid string
				var em, status, added sql.NullString
				if rows.Scan(&cid, &em, &status, &added) != nil {
					continue
				}
				if !em.Valid || em.String == "" {
					continue
				}
				key := strings.ToLower(strings.TrimSpace(em.String))
				camps := byLead[key]
				if camps == nil {
					camps = map[string]leadMembership{}
					byLead[key] = camps
				}
				camps[cid] = leadMembership{
					CampaignID:   cid,
					CampaignName: campaigns[cid].name,
					Status:       status.String,
					AddedAt:      added.String,
				}
			}
			rows.Close()
			if len(byLead) == 0 {
				return emitEmpty[dupeEntry](cmd, flags, "leads")
			}

			wantEmail := strings.ToLower(strings.TrimSpace(email))
			wantDomain := strings.ToLower(strings.TrimSpace(domain))
			var out []dupeEntry
			for em, camps := range byLead {
				switch {
				case wantEmail != "":
					if em != wantEmail {
						continue
					}
				case wantDomain != "":
					if domainOf(em) != wantDomain {
						continue
					}
				default:
					if len(camps) < 2 {
						continue
					}
				}
				memberships := make([]leadMembership, 0, len(camps))
				for _, m := range camps {
					memberships = append(memberships, m)
				}
				sort.Slice(memberships, func(i, j int) bool {
					return memberships[i].AddedAt < memberships[j].AddedAt
				})
				out = append(out, dupeEntry{
					LeadEmail:     em,
					Domain:        domainOf(em),
					CampaignCount: len(camps),
					Campaigns:     memberships,
				})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].CampaignCount != out[j].CampaignCount {
					return out[i].CampaignCount > out[j].CampaignCount
				}
				return out[i].LeadEmail < out[j].LeadEmail
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching leads.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "LEAD\tCAMPAIGNS\tIN")
			for _, e := range out {
				names := make([]string, 0, len(e.Campaigns))
				for _, m := range e.Campaigns {
					n := m.CampaignName
					if n == "" {
						n = m.CampaignID
					}
					names = append(names, n)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\n",
					truncate(e.LeadEmail, 40), e.CampaignCount, truncate(strings.Join(names, ", "), 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/smartlead-pp-cli/data.db)")
	cmd.Flags().StringVar(&email, "email", "", "Show campaign membership for one lead email")
	cmd.Flags().StringVar(&domain, "domain", "", "Show the pitch ledger for every lead at this domain")
	return cmd
}
