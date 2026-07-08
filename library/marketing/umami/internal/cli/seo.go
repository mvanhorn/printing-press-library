// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: per-site SEO acquisition report — channel mix with
// window-over-window deltas, Google share, top organic entry pages.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSeoCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	var limit int
	cmd := &cobra.Command{
		Use:   "seo [site]",
		Short: "Per-site organic report: channel mix with period-over-period deltas, Google share, top entry pages",
		Long: `Build an acquisition report for one site by joining the channel, referrer,
and entry-page breakdowns across the current and previous windows: organic
search share, Google share of referrals, channel deltas, top referrers, and
top entry pages.

Use this command for a per-site organic/referral acquisition report
(channel mix, Google share, entry pages). Do NOT use it for UTM campaign
performance; use 'campaigns' instead. Do NOT use it for a full-site traffic
summary; use 'overview' or 'digest' instead.`,
		Example:     "  umami-pp-cli seo restodom.fr --period 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build the SEO acquisition report")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			websiteID, err := resolveWebsiteID(ctx, c, args[0])
			if err != nil {
				return err
			}
			cur, err := parsePeriod(flagPeriod, time.Now())
			if err != nil {
				return usageErr(err)
			}
			prev := previousWindow(cur)
			curM, curFail := fanoutMetricsTypes(ctx, c, websiteID, cur, []string{"channel", "referrer", "entry"}, 100)
			prevM, prevFail := fanoutMetricsTypes(ctx, c, websiteID, prev, []string{"channel", "referrer"}, 100)
			if len(curFail) > 0 {
				return apiErr(fmt.Errorf("fetching current window: %s", curFail[0].Error))
			}
			prevChannels := map[string]float64{}
			for _, r := range prevM["channel"] {
				prevChannels[r.X] = r.Y
			}
			type channelRow struct {
				Channel   string  `json:"channel"`
				Visitors  float64 `json:"visitors"`
				Previous  float64 `json:"previous"`
				ChangePct float64 `json:"change_pct"`
				SharePct  float64 `json:"share_pct"`
			}
			var totalVisitors, organic float64
			for _, r := range curM["channel"] {
				totalVisitors += r.Y
				if strings.HasPrefix(strings.ToLower(r.X), "organic") {
					organic += r.Y
				}
			}
			channels := make([]channelRow, 0, len(curM["channel"]))
			for _, r := range curM["channel"] {
				share := 0.0
				if totalVisitors > 0 {
					share = r.Y / totalVisitors * 100
				}
				channels = append(channels, channelRow{
					Channel: r.X, Visitors: r.Y, Previous: prevChannels[r.X],
					ChangePct: round1(pctChange(r.Y, prevChannels[r.X])), SharePct: round1(share),
				})
			}
			var referralTotal, googleVisits float64
			prevReferrers := map[string]float64{}
			for _, r := range prevM["referrer"] {
				prevReferrers[r.X] = r.Y
			}
			type referrerRow struct {
				Referrer  string  `json:"referrer"`
				Visitors  float64 `json:"visitors"`
				ChangePct float64 `json:"change_pct"`
			}
			referrers := make([]referrerRow, 0, limit)
			for _, r := range curM["referrer"] {
				if r.X != "" {
					referralTotal += r.Y
				}
				if isGoogleDomain(r.X) {
					googleVisits += r.Y
				}
				if len(referrers) < limit {
					referrers = append(referrers, referrerRow{
						Referrer: orDirect(r.X), Visitors: r.Y,
						ChangePct: round1(pctChange(r.Y, prevReferrers[r.X])),
					})
				}
			}
			entries := curM["entry"]
			if len(entries) > limit {
				entries = entries[:limit]
			}
			googleShare := 0.0
			if referralTotal > 0 {
				googleShare = googleVisits / referralTotal * 100
			}
			organicShare := 0.0
			if totalVisitors > 0 {
				organicShare = organic / totalVisitors * 100
			}
			out := map[string]any{
				"site":            args[0],
				"website":            websiteID,
				"period":             flagPeriod,
				"organic_share_pct":  round1(organicShare),
				"google_share_pct":   round1(googleShare),
				"channels":           channels,
				"top_referrers":      referrers,
				"top_entry_pages":    entries,
			}
			if len(prevFail) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: previous-window fetch failed; deltas computed against zero\n")
				out["fetch_failures"] = prevFail
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "7d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Rows per section")
	return cmd
}
