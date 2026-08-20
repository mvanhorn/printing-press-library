// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T4 cliff.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"

	"github.com/spf13/cobra"
)

func newCliffCmd(flags *rootFlags) *cobra.Command {
	var (
		metric     string
		threshold  string
		windowFlag string
	)

	cmd := &cobra.Command{
		Use:         "cliff <site>",
		Short:       "Find the day clicks or impressions cratered, with signature hints",
		Long:        "Computes day-over-day percent delta on the chosen metric over the last <window> days from the local store and flags any day whose delta drops below --threshold. Optionally joins same-day sitemap and url-inspection deltas to surface 'signature' hints (sitemap errors increased, indexing flipped) so an agent can guess the cause.",
		Example:     "  google-search-console-pp-cli cliff sc-domain:example.com --metric clicks --threshold -25% --window 14d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			thresholdPct, err := parsePercent(threshold)
			if err != nil {
				return usageErr(err)
			}
			window, err := parseLast(windowFlag)
			if err != nil {
				return usageErr(err)
			}
			windowDays := int(window.Hours()/24) + 1

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			startDate := daysAgo(windowDays)
			db := s.DB()
			col := "clicks"
			switch metric {
			case "clicks", "impressions":
				col = metric
			default:
				return usageErr(fmt.Errorf("--metric must be clicks or impressions"))
			}
			q := fmt.Sprintf(`
SELECT date, COALESCE(SUM(%s),0) AS v
FROM search_analytics_rows
WHERE site_url = ? AND date >= ?
GROUP BY date
ORDER BY date ASC`, col)
			rows, err := db.QueryContext(ctx, q, site, startDate)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type day struct {
				date string
				v    float64
			}
			var series []day
			for rows.Next() {
				var d day
				if err := rows.Scan(&d.date, &d.v); err != nil {
					return err
				}
				series = append(series, d)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// Signature hints: sitemap errors trend per date, inspections verdict flips
			sitemapErrByDate := dailySitemapErrors(ctx, s, site)
			inspectionFlipByDate := dailyInspectionFlips(ctx, s, site)

			out := []map[string]any{}
			for i := 1; i < len(series); i++ {
				prev := series[i-1].v
				curr := series[i].v
				if prev <= 0 {
					continue
				}
				delta := (curr - prev) / prev * 100
				if delta <= thresholdPct {
					sigs := []string{}
					if v, ok := sitemapErrByDate[series[i].date]; ok && v > 0 {
						sigs = append(sigs, fmt.Sprintf("sitemap_errors_increased:%d", v))
					}
					if v, ok := inspectionFlipByDate[series[i].date]; ok && v > 0 {
						sigs = append(sigs, fmt.Sprintf("inspection_verdict_flipped:%d", v))
					}
					out = append(out, map[string]any{
						"date":         series[i].date,
						"metric_value": curr,
						"prior_value":  prev,
						"delta_pct":    delta,
						"signatures":   sigs,
					})
				}
			}
			if len(series) == 0 {
				count, _ := s.CountSearchAnalyticsRows(ctx, site)
				if count == 0 {
					return emptyStoreErr(site)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "clicks", "Metric to scan (clicks or impressions)")
	cmd.Flags().StringVar(&threshold, "threshold", "-25%", "Drop threshold (e.g. -25% means flag day-over-day declines worse than -25%)")
	cmd.Flags().StringVar(&windowFlag, "window", "7d", "How many days back to scan")
	return cmd
}

// parsePercent accepts "-25", "-25%", "25" and returns the float (sign preserved).
func parsePercent(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	return strconv.ParseFloat(s, 64)
}

// dailySitemapErrors aggregates per-date increases in sitemap errors. Best-effort —
// returns an empty map on error.
func dailySitemapErrors(ctx context.Context, s *store.Store, site string) map[string]int64 {
	out := map[string]int64{}
	rows, err := s.DB().QueryContext(ctx, `
SELECT DATE(snapshot_at) AS d, SUM(errors) AS e
FROM sitemaps WHERE site_url = ?
GROUP BY DATE(snapshot_at)
ORDER BY d ASC`, site)
	if err != nil {
		return out
	}
	defer rows.Close()
	prev := int64(-1)
	for rows.Next() {
		var d string
		var e int64
		if rows.Scan(&d, &e) != nil {
			continue
		}
		if prev >= 0 && e > prev {
			out[d] = e - prev
		}
		prev = e
	}
	return out
}

// dailyInspectionFlips counts URLs whose verdict changed on each date.
func dailyInspectionFlips(ctx context.Context, s *store.Store, site string) map[string]int64 {
	out := map[string]int64{}
	rows, err := s.DB().QueryContext(ctx, `
SELECT DATE(snapshot_at) AS d, COUNT(*) AS c
FROM url_inspections WHERE site_url = ?
GROUP BY DATE(snapshot_at)
ORDER BY d ASC`, site)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var c int64
		if rows.Scan(&d, &c) == nil {
			out[d] = c
		}
	}
	return out
}
