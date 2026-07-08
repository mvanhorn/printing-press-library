// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: month pacing. Projects month-to-date traffic to month-end
// and compares against the prior full month, per site.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelPaceCmd(flags *rootFlags) *cobra.Command {
	var siteArg string
	cmd := &cobra.Command{
		Use:   "pace",
		Short: "Month-to-date traffic projected to month-end vs the prior full month, per site",
		Long: `Project each site's month-to-date visitors to month-end using the
elapsed-day ratio and compare the projection against the prior full month.
Verdict per site: ahead, on-track (within ±5%), or behind.

Use this command for a month-to-date projection against the prior full
month. Do NOT use it for comparing two completed periods with growth %;
use 'websites stats' with --compare instead.`,
		Example:     "  umami-pp-cli pace --json\n  umami-pp-cli pace --site restodom.fr",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--site=f1fa838a-fc6a-49c1-bdef-c3688dc92574"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would project month-to-date traffic against the prior month")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var sites []umamiSite
			if siteArg != "" {
				id, err := resolveWebsiteID(ctx, c, siteArg)
				if err != nil {
					return err
				}
				sites = []umamiSite{{ID: id, Domain: siteArg}}
			} else {
				sites, err = fetchAllWebsites(ctx, c)
				if err != nil {
					return err
				}
			}
			now := time.Now()
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			prevStart := monthStart.AddDate(0, -1, 0)
			daysInMonth := monthStart.AddDate(0, 1, -1).Day()
			elapsed := now.Sub(monthStart).Hours() / 24
			if elapsed < 0.04 {
				elapsed = 0.04 // first minutes of the month: avoid wild projections
			}
			mtd := periodWindow{ms(monthStart), ms(now)}
			prevMonth := periodWindow{ms(prevStart), ms(monthStart)}
			type paceRow struct {
				Domain        string  `json:"domain"`
				ID            string  `json:"id"`
				MTDVisitors   float64 `json:"mtd_visitors"`
				Projected     float64 `json:"projected_month_end"`
				PriorMonth    float64 `json:"prior_month"`
				ProjectedPct  float64 `json:"projected_change_pct"`
				Verdict       string  `json:"verdict"`
			}
			rows := make([]paceRow, 0, len(sites))
			failures := make([]fetchFailure, 0)
			for _, site := range sites {
				cur, err := getStats(ctx, c, site.ID, mtd, "")
				if err != nil {
					failures = append(failures, fetchFailure{ID: site.Domain, Error: err.Error()})
					continue
				}
				prior, err := getStats(ctx, c, site.ID, prevMonth, "")
				if err != nil {
					failures = append(failures, fetchFailure{ID: site.Domain + " (prior month)", Error: err.Error()})
					continue
				}
				projected := cur.Visitors * float64(daysInMonth) / elapsed
				change := pctChange(projected, prior.Visitors)
				verdict := "on-track"
				switch {
				case prior.Visitors == 0 && cur.Visitors > 0:
					verdict = "new"
				case change > 5:
					verdict = "ahead"
				case change < -5:
					verdict = "behind"
				}
				rows = append(rows, paceRow{
					Domain: site.Domain, ID: site.ID,
					MTDVisitors: cur.Visitors, Projected: round1(projected),
					PriorMonth: prior.Visitors, ProjectedPct: round1(change), Verdict: verdict,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].ProjectedPct > rows[j].ProjectedPct })
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sites had failed fetches; pacing covers the remaining %d\n", len(failures), len(sites), len(rows))
			}
			out := map[string]any{
				"month":          monthStart.Format("2006-01"),
				"elapsed_days":   round1(elapsed),
				"days_in_month":  daysInMonth,
				"sites":          rows,
				"fetch_failures": failures,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&siteArg, "site", "", "Limit to one website (UUID or domain); default all")
	return cmd
}
