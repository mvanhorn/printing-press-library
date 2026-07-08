// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: UTM campaign performance, absorbed from frontedu's
// get_campaign_performance MCP tool, using v3's native utm* metric types.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newCampaignsCmd(flags *rootFlags) *cobra.Command {
	var period string
	var limit int
	cmd := &cobra.Command{
		Use:   "campaigns [site]",
		Short: "UTM campaign performance: sources, mediums, and campaigns ranked by visitors",
		Long: `Fetch the utm_source, utm_medium, and utm_campaign breakdowns for one site
in a single command.

Use this command for UTM campaign performance. Do NOT use it for the
organic/referral channel mix; use 'seo' instead.`,
		Example:     "  umami-pp-cli campaigns restodom.fr --period 30d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch UTM source/medium/campaign breakdowns")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
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
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			metrics, failures := fanoutMetricsTypes(ctx, c, websiteID, w,
				[]string{"utmSource", "utmMedium", "utmCampaign"}, limit)
			view := map[string]any{
				"website":   websiteID,
				"period":    period,
				"sources":   metrics["utmSource"],
				"mediums":   metrics["utmMedium"],
				"campaigns": metrics["utmCampaign"],
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 3 UTM fetches failed; view assembled from the rest\n", len(failures))
				view["fetch_failures"] = failures
			}
			total := 0
			for _, rows := range metrics {
				total += len(rows)
			}
			if total == 0 && len(failures) == 0 {
				view["note"] = "no UTM-tagged traffic in this period"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "30d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Rows per breakdown")
	return cmd
}
