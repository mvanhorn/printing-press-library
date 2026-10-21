// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type dailyMetricResponse struct {
	DailyMetrics []struct {
		Metrics map[string]any `json:"metrics"`
	} `json:"daily_metrics"`
}

type perfPeriod struct {
	StartDate string             `json:"start_date"`
	EndDate   string             `json:"end_date"`
	Totals    map[string]float64 `json:"totals"`
}

type perfMetricDelta struct {
	Metric   string   `json:"metric"`
	Current  float64  `json:"current"`
	Prior    float64  `json:"prior"`
	Delta    float64  `json:"delta"`
	DeltaPct *float64 `json:"delta_pct"`
}

func compareWindows(end time.Time, days int) (time.Time, time.Time, time.Time, time.Time) {
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	currentStart := end.AddDate(0, 0, -days+1)
	priorEnd := currentStart.AddDate(0, 0, -1)
	priorStart := priorEnd.AddDate(0, 0, -days+1)
	return currentStart, end, priorStart, priorEnd
}

func sumDailyMetrics(raw json.RawMessage, metrics []string) (map[string]float64, error) {
	var split map[string]dailyMetricResponse
	if err := json.Unmarshal(raw, &split); err != nil {
		return nil, fmt.Errorf("parsing account analytics: %w", err)
	}
	keys := []string{"all"}
	if _, ok := split["all"]; !ok {
		keys = keys[:0]
		for key := range split {
			keys = append(keys, key)
		}
	}
	totals := make(map[string]float64, len(metrics))
	for _, key := range keys {
		for _, day := range split[key].DailyMetrics {
			for _, metric := range metrics {
				if v, ok := day.Metrics[metric].(float64); ok {
					totals[metric] += v
				}
			}
		}
	}
	return totals, nil
}

func buildPerfComparison(days int, currentStart, currentEnd, priorStart, priorEnd time.Time, current, prior map[string]float64, metrics []string) map[string]any {
	deltas := make([]perfMetricDelta, 0, len(metrics))
	for _, metric := range metrics {
		c, p := current[metric], prior[metric]
		row := perfMetricDelta{Metric: metric, Current: c, Prior: p, Delta: c - p}
		if p != 0 {
			pct := math.Round((c-p)/p*1000) / 10
			row.DeltaPct = &pct
		}
		deltas = append(deltas, row)
	}
	period := func(start, end time.Time, totals map[string]float64) perfPeriod {
		return perfPeriod{start.Format("2006-01-02"), end.Format("2006-01-02"), totals}
	}
	return map[string]any{"days": days, "current": period(currentStart, currentEnd, current), "prior": period(priorStart, priorEnd, prior), "metrics": deltas}
}

func newNovelPerfCompareCmd(flags *rootFlags) *cobra.Command {
	var days int
	var metricsRaw, endDate, adAccountID string
	cmd := &cobra.Command{Use: "compare", Short: "Compare account totals across two periods", Long: "Use this command for account-level totals compared across two periods.\nDo NOT use this command for per-Pin rankings or save/click rates; use 'pins funnel' instead.", Example: strings.Trim(`
  pinterest-pp-cli perf compare --days=7
`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--days=7"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "perf compare")
		}
		if days < 1 || days > 45 {
			return usageErr(fmt.Errorf("--days must be between 1 and 45"))
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return usageErr(fmt.Errorf("--end-date must use YYYY-MM-DD"))
		}
		end = end.UTC()
		cs, ce, ps, pe := compareWindows(end, days)
		cutoff := nowUTC().UTC()
		cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -90)
		if ps.Before(cutoff) {
			return usageErr(fmt.Errorf("--days and --end-date make the start date older than the 90-day analytics window"))
		}
		metrics := splitCSV(metricsRaw)
		if len(metrics) == 0 {
			return usageErr(fmt.Errorf("--metrics must contain at least one metric"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		fetch := func(start, end time.Time) (map[string]float64, error) {
			path := appendArrayQueryParam("/user_account/analytics", "metric_types", metricsRaw, "form", false)
			params := map[string]string{"start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}
			if adAccountID != "" {
				params["ad_account_id"] = adAccountID
			}
			raw, err := c.Get(ctx, path, params)
			if err != nil {
				return nil, classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return sumDailyMetrics(raw, metrics)
		}
		current, err := fetch(cs, ce)
		if err != nil {
			return err
		}
		prior, err := fetch(ps, pe)
		if err != nil {
			return err
		}
		result := buildPerfComparison(days, cs, ce, ps, pe, current, prior, metrics)
		w := cmd.OutOrStdout()
		if !wantsHumanTable(w, flags) {
			return printJSONFiltered(w, result, flags)
		}
		fmt.Fprintf(w, "%-20s %12s %12s %12s %8s\n", "METRIC", "CURRENT", "PRIOR", "DELTA", "DELTA%")
		for _, row := range result["metrics"].([]perfMetricDelta) {
			pct := "null"
			if row.DeltaPct != nil {
				pct = fmt.Sprintf("%.1f%%", *row.DeltaPct)
			}
			fmt.Fprintf(w, "%-20s %12.2f %12.2f %12.2f %8s\n", row.Metric, row.Current, row.Prior, row.Delta, pct)
		}
		return nil
	}}
	cmd.Flags().IntVar(&days, "days", 28, "Number of days in each comparison window (1-45)")
	cmd.Flags().StringVar(&metricsRaw, "metrics", "IMPRESSION,SAVE,PIN_CLICK,OUTBOUND_CLICK,ENGAGEMENT", "Comma-separated analytics metrics to compare")
	cmd.Flags().StringVar(&endDate, "end-date", nowUTC().AddDate(0, 0, -1).Format("2006-01-02"), "End date for current period in YYYY-MM-DD format")
	cmd.Flags().StringVar(&adAccountID, "ad-account-id", "", "Optional ad account identifier for analytics")
	return cmd
}
