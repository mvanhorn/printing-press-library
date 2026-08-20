// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/internal/store"
	"github.com/spf13/cobra"
)

type statsRollupRow struct {
	Bucket   string  `json:"bucket"`
	Metric   string  `json:"metric"`
	Value    float64 `json:"value"`
	DeltaAbs float64 `json:"delta_abs"`
	DeltaPct float64 `json:"delta_pct"`
}

func newStatsRollupCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagMetrics string
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "rollup",
		Short: "Aggregate stats by day/week/month with WoW/MoM deltas",
		Long: `Fetches stats from GET /v3/stats for the requested window, stores daily data
in the local SQLite store as stats_daily, then aggregates locally by
day|week|month bucket. Computes WoW delta (last full bucket vs prior bucket):
both absolute value and percentage. Output table: bucket, metric, value, delta_abs, delta_pct.`,
		Example:     "  sendgrid-pp-cli stats rollup --by week --metric opens,clicks,bounces --window 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			metrics := parseMetrics(flagMetrics)
			if len(metrics) == 0 {
				return usageErr(fmt.Errorf("--metric is required"))
			}

			windowDays, err := parseWindowDays(flagWindow)
			if err != nil {
				return usageErr(err)
			}

			switch flagBy {
			case "day", "week", "month":
			default:
				return usageErr(fmt.Errorf("--by must be day, week, or month"))
			}

			endDate := time.Now().UTC()
			startDate := endDate.AddDate(0, 0, -windowDays)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch stats
			data, err := c.Get("/v3/stats", map[string]string{
				"start_date":    startDate.Format("2006-01-02"),
				"end_date":      endDate.Format("2006-01-02"),
				"aggregated_by": "day",
				"limit":         "500",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Store daily data
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("sendgrid-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = db.Close() }()
			_ = db.Upsert("stats_daily", "latest", data)

			// Parse stats array
			var statDays []map[string]json.RawMessage
			if err := json.Unmarshal(data, &statDays); err != nil {
				return fmt.Errorf("parsing stats: %w", err)
			}

			// Build per-day per-metric values
			type dayMetric struct {
				date   string
				metric string
				value  float64
			}
			var dayMetrics []dayMetric

			for _, day := range statDays {
				dateRaw, ok := day["date"]
				if !ok {
					continue
				}
				var date string
				_ = json.Unmarshal(dateRaw, &date)

				statsRaw, ok := day["stats"]
				if !ok {
					continue
				}
				var stats []map[string]json.RawMessage
				if err := json.Unmarshal(statsRaw, &stats); err != nil {
					continue
				}

				for _, stat := range stats {
					metricsRaw, ok := stat["metrics"]
					if !ok {
						continue
					}
					var metricsMap map[string]json.RawMessage
					if err := json.Unmarshal(metricsRaw, &metricsMap); err != nil {
						continue
					}
					for _, metric := range metrics {
						if valRaw, ok := metricsMap[metric]; ok {
							var v float64
							_ = json.Unmarshal(valRaw, &v)
							dayMetrics = append(dayMetrics, dayMetric{date: date, metric: metric, value: v})
						}
					}
				}
			}

			// Aggregate by bucket
			type bucketKey struct {
				bucket string
				metric string
			}
			bucketTotals := map[bucketKey]float64{}
			var bucketOrder []string
			bucketSeen := map[string]bool{}

			for _, dm := range dayMetrics {
				bucket := dateToBucket(dm.date, flagBy)
				k := bucketKey{bucket: bucket, metric: dm.metric}
				bucketTotals[k] += dm.value
				if !bucketSeen[bucket] {
					bucketSeen[bucket] = true
					bucketOrder = append(bucketOrder, bucket)
				}
			}

			// Build output rows with delta
			var rows []statsRollupRow
			for _, bucket := range bucketOrder {
				prevBucket := prevBucketFor(bucket, flagBy)
				for _, metric := range metrics {
					val := bucketTotals[bucketKey{bucket: bucket, metric: metric}]
					prev := bucketTotals[bucketKey{bucket: prevBucket, metric: metric}]
					deltaAbs := val - prev
					var deltaPct float64
					if prev != 0 {
						deltaPct = (deltaAbs / prev) * 100
					}
					rows = append(rows, statsRollupRow{
						Bucket:   bucket,
						Metric:   metric,
						Value:    val,
						DeltaAbs: deltaAbs,
						DeltaPct: deltaPct,
					})
				}
			}

			if flags.asJSON {
				raw, _ := json.Marshal(rows)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			if len(rows) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No stats data found for the window.")
				return nil
			}

			// Render the table with an explicit column order so identifier
			// columns (bucket, metric) precede their derived deltas. The
			// generic printAutoTable sorts unclassified keys arbitrarily,
			// which placed delta_abs/delta_pct before bucket/metric and
			// made the table hard to read left-to-right.
			tw := newTabWriter(cmd.OutOrStdout())
			_, _ = fmt.Fprintln(tw, strings.Join([]string{
				bold("BUCKET"), bold("METRIC"), bold("VALUE"),
				bold("DELTA_ABS"), bold("DELTA_PCT"),
			}, "\t"))
			for _, r := range rows {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					r.Bucket,
					r.Metric,
					fmt.Sprintf("%.0f", r.Value),
					fmt.Sprintf("%+.0f", r.DeltaAbs),
					fmt.Sprintf("%+.1f%%", r.DeltaPct),
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagBy, "by", "day", "Bucket size: day|week|month")
	cmd.Flags().StringVar(&flagMetrics, "metric", "opens,clicks,bounces", "Comma-separated metrics to aggregate")
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Time window: 7d|30d|90d or any Nd")

	return cmd
}

func parseMetrics(s string) []string {
	var out []string
	for _, m := range strings.Split(s, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}

func parseWindowDays(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasSuffix(s, "d") {
		return 0, fmt.Errorf("--window must end in 'd' (e.g. 7d, 30d, 90d); got %q", s)
	}
	var n int
	_, err := fmt.Sscanf(s[:len(s)-1], "%d", &n)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid window %q", s)
	}
	return n, nil
}

func dateToBucket(date, by string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	switch by {
	case "week":
		// ISO week bucket: Monday of the week
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		return monday.Format("2006-01-02")
	case "month":
		return t.Format("2006-01")
	default: // day
		return date
	}
}

func prevBucketFor(bucket, by string) string {
	switch by {
	case "week":
		t, err := time.Parse("2006-01-02", bucket)
		if err != nil {
			return ""
		}
		return t.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		t, err := time.Parse("2006-01", bucket)
		if err != nil {
			return ""
		}
		return t.AddDate(0, -1, 0).Format("2006-01")
	default: // day
		t, err := time.Parse("2006-01-02", bucket)
		if err != nil {
			return ""
		}
		return t.AddDate(0, 0, -1).Format("2006-01-02")
	}
}
