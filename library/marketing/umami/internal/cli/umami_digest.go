// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: cron-friendly markdown/JSON digest, absorbed from
// Thunderbottom/umami-alerts and the n8n weekly-summary template.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newDigestCmd(flags *rootFlags) *cobra.Command {
	var period string
	var all bool
	cmd := &cobra.Command{
		Use:   "digest [site]",
		Short: "Traffic digest for one site or the whole portfolio — markdown by default, Slack/email-ready",
		Long: `Build a traffic digest: summary stats with previous-period comparison,
top pages, and top referrers. Prints markdown by default (pipe it to Slack,
email, or a report); use --json for structured output. With --all, one
digest section per website.

Use this command for a scheduled or shareable summary. Do NOT use it for
acquisition analysis; use 'seo' instead.`,
		Example:     "  umami-pp-cli digest restodom.fr --period 7d\n  umami-pp-cli digest --all --period 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a traffic digest")
				return nil
			}
			if len(args) == 0 && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("pass a site (UUID or domain) or --all"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			var sites []umamiSite
			if all {
				sites, err = fetchAllWebsites(ctx, c)
				if err != nil {
					return err
				}
			} else {
				id, err := resolveWebsiteID(ctx, c, args[0])
				if err != nil {
					return err
				}
				allSites, err := fetchAllWebsites(ctx, c)
				if err != nil {
					return err
				}
				for _, s := range allSites {
					if s.ID == id {
						sites = []umamiSite{s}
						break
					}
				}
				if len(sites) == 0 {
					sites = []umamiSite{{ID: id, Domain: args[0], Name: args[0]}}
				}
			}
			type siteDigest struct {
				Site         umamiSite        `json:"site"`
				Stats        map[string]any   `json:"stats"`
				TopPages     []umamiMetricRow `json:"top_pages"`
				TopReferrers []umamiMetricRow `json:"top_referrers"`
			}
			digests := make([]siteDigest, 0, len(sites))
			failures := make([]fetchFailure, 0)
			for _, site := range sites {
				stats, err := getStats(ctx, c, site.ID, w, "prev")
				if err != nil {
					failures = append(failures, fetchFailure{ID: site.Domain, Error: err.Error()})
					continue
				}
				metrics, mFail := fanoutMetricsTypes(ctx, c, site.ID, w, []string{"path", "referrer"}, 5)
				if len(mFail) > 0 {
					failures = append(failures, fetchFailure{ID: site.Domain + " (metrics)", Error: mFail[0].Error})
				}
				sm := map[string]any{
					"pageviews": stats.Pageviews, "visitors": stats.Visitors,
					"visits": stats.Visits, "bounces": stats.Bounces,
				}
				if stats.Comparison != nil {
					sm["visitors_change_pct"] = round1(pctChange(stats.Visitors, stats.Comparison.Visitors))
					sm["pageviews_change_pct"] = round1(pctChange(stats.Pageviews, stats.Comparison.Pageviews))
				}
				digests = append(digests, siteDigest{Site: site, Stats: sm,
					TopPages: metrics["path"], TopReferrers: metrics["referrer"]})
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sites had failed fetches; digest covers the remaining %d\n", len(failures), len(sites), len(digests))
			}
			if wantsMachineOutput(flags) {
				out := map[string]any{"period": period, "digests": digests, "fetch_failures": failures}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			var b strings.Builder
			fmt.Fprintf(&b, "# Umami digest — last %s\n\n", period)
			for _, d := range digests {
				fmt.Fprintf(&b, "## %s (%s)\n\n", d.Site.Name, d.Site.Domain)
				if ch, ok := d.Stats["visitors_change_pct"].(float64); ok {
					fmt.Fprintf(&b, "- Visitors: %.0f (%+.1f%%) · Pageviews: %.0f (%+.1f%%)\n",
						d.Stats["visitors"], ch, d.Stats["pageviews"], d.Stats["pageviews_change_pct"])
				} else {
					fmt.Fprintf(&b, "- Visitors: %.0f · Pageviews: %.0f\n", d.Stats["visitors"], d.Stats["pageviews"])
				}
				if len(d.TopPages) > 0 {
					fmt.Fprintf(&b, "- Top pages:\n")
					for _, p := range d.TopPages {
						fmt.Fprintf(&b, "    - %s (%.0f)\n", p.X, p.Y)
					}
				}
				if len(d.TopReferrers) > 0 {
					fmt.Fprintf(&b, "- Top referrers:\n")
					for _, r := range d.TopReferrers {
						fmt.Fprintf(&b, "    - %s (%.0f)\n", orDirect(r.X), r.Y)
					}
				}
				fmt.Fprintln(&b)
			}
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&period, "period", "7d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().BoolVar(&all, "all", false, "Digest every website")
	return cmd
}

func orDirect(s string) string {
	if s == "" {
		return "(direct)"
	}
	return s
}
