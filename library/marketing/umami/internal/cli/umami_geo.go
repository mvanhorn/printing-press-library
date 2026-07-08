// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: geo drill-down with share %, absorbed from
// frontedu's get_geo_insights MCP tool.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newGeoCmd(flags *rootFlags) *cobra.Command {
	var period string
	var limit int
	cmd := &cobra.Command{
		Use:   "geo [site]",
		Short: "Geographic drill-down: countries, regions, and cities with visitor share %",
		Example:     "  umami-pp-cli geo restodom.fr --period 30d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch country/region/city breakdowns")
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
			metrics, failures := fanoutMetricsTypes(ctx, c, websiteID, w, []string{"country", "region", "city"}, limit)
			type geoRow struct {
				Name     string  `json:"name"`
				Visitors float64 `json:"visitors"`
				SharePct float64 `json:"share_pct"`
			}
			withShare := func(rows []umamiMetricRow) []geoRow {
				var total float64
				for _, r := range rows {
					total += r.Y
				}
				out := make([]geoRow, 0, len(rows))
				for _, r := range rows {
					share := 0.0
					if total > 0 {
						share = r.Y / total * 100
					}
					out = append(out, geoRow{Name: r.X, Visitors: r.Y, SharePct: round1(share)})
				}
				return out
			}
			view := map[string]any{
				"website":   websiteID,
				"period":    period,
				"countries": withShare(metrics["country"]),
				"regions":   withShare(metrics["region"]),
				"cities":    withShare(metrics["city"]),
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 3 geo fetches failed; view assembled from the rest\n", len(failures))
				view["fetch_failures"] = failures
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "30d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Rows per level")
	return cmd
}
