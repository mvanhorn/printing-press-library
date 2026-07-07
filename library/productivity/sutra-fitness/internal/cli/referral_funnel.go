// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: referral funnel conversion.
//
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type referralFunnel struct {
	Referrals    int     `json:"referrals"`
	SignedUp     int     `json:"signed_up"`
	Purchased    int     `json:"purchased"`
	Attended     int     `json:"attended"`
	SignupRate   float64 `json:"signup_rate"`
	PurchaseRate float64 `json:"purchase_rate"`
	AttendRate   float64 `json:"attend_rate"`
}

type topReferrer struct {
	ReferrerID string `json:"referrer_id"`
	Name       string `json:"name"`
	Referrals  int    `json:"referrals"`
}

type referralFunnelView struct {
	Funnel       referralFunnel `json:"funnel"`
	TopReferrers []topReferrer  `json:"top_referrers"`
}

func newNovelReferralFunnelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "referral-funnel",
		Short: "Trace referrals to whether the referred client signed up, purchased, and attended, and rank top referrers.",
		Long: `Measure referral-program conversion from your locally synced data.

Walks referrals to the referred client, then to whether that client purchased
and attended (checked in), reporting funnel counts and conversion rates plus
your top referrers. This three-table funnel is not exposed by any single Sutra
endpoint.

Run 'sutra-fitness-pp-cli sync' first to populate referrals, clients, purchases,
and reservations.`,
		Example:     "  sutra-fitness-pp-cli referral-funnel --json\n  sutra-fitness-pp-cli referral-funnel --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, referralFunnelView{TopReferrers: []topReferrer{}})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "referrals") {
				hintIfStale(cmd, db, "referrals", flags.maxAge)
			}

			set := func(query string) (map[string]bool, error) {
				rows, err := db.DB().QueryContext(cmd.Context(), query)
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				m := map[string]bool{}
				for rows.Next() {
					var v sql.NullString
					if err := rows.Scan(&v); err != nil {
						continue
					}
					if v.Valid && v.String != "" {
						m[v.String] = true
					}
				}
				return m, rows.Err()
			}

			existingClients, err := set(`SELECT id FROM clients WHERE COALESCE(removed,0)=0`)
			if err != nil {
				return fmt.Errorf("querying clients: %w", err)
			}
			// purchasers/attendees consistently exclude removed clients, matching
			// existingClients, so every funnel stage uses the same client universe.
			purchasers, err := set(`SELECT DISTINCT p.client_id FROM purchases p JOIN clients cl ON p.client_id = cl.id WHERE COALESCE(cl.removed,0)=0 AND p.client_id IS NOT NULL`)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}
			attendees, err := set(`SELECT DISTINCT json_extract(r.data,'$.client_id') FROM reservations r JOIN clients cl ON json_extract(r.data,'$.client_id') = cl.id WHERE COALESCE(cl.removed,0)=0 AND (json_extract(r.data,'$.status') IN ('ATTENDED','CHECKED_IN') OR json_extract(r.data,'$.checked_in')=1)`)
			if err != nil {
				return fmt.Errorf("querying reservations: %w", err)
			}

			// Client display names for top referrers.
			names := map[string]string{}
			nameRows, err := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(first_name,''), COALESCE(last_name,'') FROM clients`)
			if err != nil {
				return fmt.Errorf("querying client names: %w", err)
			}
			for nameRows.Next() {
				var id, first, last string
				if err := nameRows.Scan(&id, &first, &last); err != nil {
					continue
				}
				names[id] = strings.TrimSpace(first + " " + last)
			}
			_ = nameRows.Close()

			refRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(referred_user_id,''), COALESCE(referring_user_id,'') FROM referrals`)
			if err != nil {
				return fmt.Errorf("querying referrals: %w", err)
			}
			defer refRows.Close()

			funnel := referralFunnel{}
			referrerCounts := map[string]int{}
			for refRows.Next() {
				var referred, referring string
				if err := refRows.Scan(&referred, &referring); err != nil {
					continue
				}
				funnel.Referrals++
				if referred != "" && existingClients[referred] {
					funnel.SignedUp++
					if purchasers[referred] {
						funnel.Purchased++
					}
					if attendees[referred] {
						funnel.Attended++
					}
				}
				if referring != "" {
					referrerCounts[referring]++
				}
			}
			if err := refRows.Err(); err != nil {
				return fmt.Errorf("iterating referrals: %w", err)
			}

			funnel.SignupRate = pct(funnel.SignedUp, funnel.Referrals)
			funnel.PurchaseRate = pct(funnel.Purchased, funnel.Referrals)
			funnel.AttendRate = pct(funnel.Attended, funnel.Referrals)

			top := make([]topReferrer, 0, len(referrerCounts))
			for id, n := range referrerCounts {
				name := names[id]
				if name == "" {
					name = "(unknown)"
				}
				top = append(top, topReferrer{ReferrerID: id, Name: name, Referrals: n})
			}
			sort.SliceStable(top, func(i, j int) bool {
				if top[i].Referrals != top[j].Referrals {
					return top[i].Referrals > top[j].Referrals
				}
				return top[i].Name < top[j].Name
			})
			if limit > 0 && len(top) > limit {
				top = top[:limit]
			}

			return emitAnalytics(cmd, flags, referralFunnelView{Funnel: funnel, TopReferrers: top})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum top referrers to return (0 = all)")
	return cmd
}
