// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var regions string
	var metric string
	var period int

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Aggregate-trends across multiple regions in long format.",
		Long: `For each region in --regions, calls /aggregate-trends and emits a tidy
long table of (region × month × metric × value). Pass --metric to scope
to one metric, or omit to receive every supported metric.`,
		Example:     `  redfin-pp-cli trends --regions 30772:6,29470:6 --metric median-sale --period 24 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if regions == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("--regions required (comma-separated slugs or id:type pairs)"))
			}
			parts := strings.Split(regions, ",")
			parsed := make([]struct {
				id    int64
				typ   int
				label string
			}, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				id, typ, err := parseRegionSlug(p)
				if err != nil {
					return usageErr(err)
				}
				parsed = append(parsed, struct {
					id    int64
					typ   int
					label string
				}{id: id, typ: typ, label: strconv.FormatInt(id, 10)})
			}
			if dryRunOK(flags) {
				for _, p := range parsed {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: "+marketDryRunPath(p.id, p.typ, period))
				}
				return nil
			}
			wantMetric := metricKey(metric)
			var out []redfin.RegionTrendPoint
			ok, failed := 0, 0
			for _, p := range parsed {
				rows, err := fetchMarketTrends(flags, p.id, p.typ, period, p.label)
				if err != nil {
					failed++
					fmt.Fprintf(cmd.ErrOrStderr(), "trends region %s: ERROR %v\n", p.label, err)
					continue
				}
				rowsKept := 0
				for _, r := range rows {
					if wantMetric != "" && r.Metric != wantMetric {
						continue
					}
					out = append(out, r)
					rowsKept++
				}
				ok++
				fmt.Fprintf(cmd.ErrOrStderr(), "trends region %s: %d row(s)\n", p.label, rowsKept)
			}
			if out == nil {
				out = []redfin.RegionTrendPoint{}
			}
			if werr := printJSONFiltered(cmd.OutOrStdout(), out, flags); werr != nil {
				return werr
			}
			if ok == 0 && failed > 0 {
				return apiErr(fmt.Errorf("trends: 0 of %d regions returned data (all %d failed)", failed, failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&regions, "regions", "", "Comma-separated region slugs/ids (REQUIRED)")
	cmd.Flags().StringVar(&metric, "metric", "", "Metric filter: median-sale|median-list|median-sale-per-sqft|median-list-per-sqft|avg-days-on-market|median-dom|active-count|avg-num-offers|yoy-sale-per-sqft-pct|yoy-median-sale-pct (omit to return all)")
	cmd.Flags().IntVar(&period, "period", 24, "Window in months")
	return cmd
}

func metricKey(s string) string {
	switch strings.ToLower(s) {
	case "median-sale", "median_sale":
		return "median_sale"
	case "median-list", "median_list":
		return "median_list"
	case "median-sale-per-sqft", "median_sale_per_sqft":
		return "median_sale_per_sqft"
	case "median-list-per-sqft", "median_list_per_sqft":
		return "median_list_per_sqft"
	case "avg-days-on-market", "avg_days_on_market":
		return "avg_days_on_market"
	case "median-dom", "median_dom":
		return "median_dom"
	case "dom":
		return "median_dom"
	case "active-count", "active_count":
		return "active_count"
	case "avg-num-offers", "avg_num_offers":
		return "avg_num_offers"
	case "yoy-sale-per-sqft-pct", "yoy_sale_per_sqft_pct":
		return "yoy_sale_per_sqft_pct"
	case "yoy-median-sale-pct", "yoy_median_sale_pct":
		return "yoy_median_sale_pct"
	}
	return ""
}
