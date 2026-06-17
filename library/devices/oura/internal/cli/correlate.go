// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type correlateView struct {
	Tags             []string `json:"tags"`
	Metric           string   `json:"metric"`
	Since            string   `json:"since"`
	TaggedDayCount   int      `json:"tagged_day_count"`
	TaggedAvgDelta   float64  `json:"tagged_next_day_avg_delta,omitempty"`
	BaselineDayCount int      `json:"baseline_day_count"`
	BaselineAvgDelta float64  `json:"baseline_next_day_avg_delta,omitempty"`
	Verdict          string   `json:"verdict"`
	Note             string   `json:"note,omitempty"`
}

func newNovelCorrelateCmd(flags *rootFlags) *cobra.Command {
	var flagTag []string
	var flagMetric string
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "correlate",
		Short: "Find which habits actually move your scores: correlate any enhanced tag string with the next-day change in sleep",
		Long: `Correlates days you logged a given tag against the next-day change in a
metric, compared against the same next-day change on untagged days. This
treats tags as a predictor variable instead of just a display field.`,
		Example: `  oura-pp-cli correlate --tag alcohol --metric readiness
  oura-pp-cli correlate --tag "late meal" --tag travel --metric sleep --since 90d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would correlate --tag day occurrences with next-day change in --metric")
				return nil
			}
			if len(flagTag) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tag is required (repeatable)"))
			}
			metricName := flagMetric
			if metricName == "" {
				metricName = "readiness"
			}
			spec, err := resolveMetric(metricName)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			start, err := resolveSinceDay(flagSince, 90)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			end := today()

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprint(cmd.ErrOrStderr(), missingMirrorMessage(dbPath))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			taggedDays, err := tagDays(db, flagTag, start, end)
			if err != nil {
				return fmt.Errorf("querying tags: %w", err)
			}
			series, err := metricSeries(db, spec, start, addDays(end, 1))
			if err != nil {
				return fmt.Errorf("querying %s: %w", metricName, err)
			}

			var taggedDeltas, baselineDeltas []float64
			for d := start; d <= end; d = addDays(d, 1) {
				v, ok := series[d]
				vNext, okNext := series[addDays(d, 1)]
				if !ok || !okNext {
					continue
				}
				delta := vNext - v
				if taggedDays[d] {
					taggedDeltas = append(taggedDeltas, delta)
				} else {
					baselineDeltas = append(baselineDeltas, delta)
				}
			}

			view := correlateView{
				Tags:             flagTag,
				Metric:           metricName,
				Since:            start,
				TaggedDayCount:   len(taggedDeltas),
				BaselineDayCount: len(baselineDeltas),
			}
			if len(taggedDeltas) > 0 {
				m, _ := meanStdDev(taggedDeltas)
				view.TaggedAvgDelta = round2(m)
			}
			if len(baselineDeltas) > 0 {
				m, _ := meanStdDev(baselineDeltas)
				view.BaselineAvgDelta = round2(m)
			}

			switch {
			case len(taggedDeltas) < 3:
				view.Verdict = "insufficient-data"
				view.Note = fmt.Sprintf("only %d tagged day(s) with both a value and a next-day value — log this tag on more days for a reliable read", len(taggedDeltas))
			default:
				diff := view.TaggedAvgDelta - view.BaselineAvgDelta
				switch {
				case diff <= -1:
					view.Verdict = fmt.Sprintf("%s tends to precede a worse next-day %s (%.1f vs %.1f baseline)", strings.Join(flagTag, "/"), metricName, view.TaggedAvgDelta, view.BaselineAvgDelta)
				case diff >= 1:
					view.Verdict = fmt.Sprintf("%s tends to precede a better next-day %s (%.1f vs %.1f baseline)", strings.Join(flagTag, "/"), metricName, view.TaggedAvgDelta, view.BaselineAvgDelta)
				default:
					view.Verdict = fmt.Sprintf("no clear effect of %s on next-day %s (%.1f vs %.1f baseline)", strings.Join(flagTag, "/"), metricName, view.TaggedAvgDelta, view.BaselineAvgDelta)
				}
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, view.Verdict)
			fmt.Fprintf(out, "  tagged days:   %d (avg next-day delta %.2f)\n", view.TaggedDayCount, view.TaggedAvgDelta)
			fmt.Fprintf(out, "  baseline days: %d (avg next-day delta %.2f)\n", view.BaselineDayCount, view.BaselineAvgDelta)
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&flagTag, "tag", nil, "Tag text to match against logged enhanced tags (repeatable; matches tag_type_code or comment, case-insensitive)")
	cmd.Flags().StringVar(&flagMetric, "metric", "readiness", "Metric whose next-day change to correlate against: "+joinMetrics())
	cmd.Flags().StringVar(&flagSince, "since", "", "Start of the window: a duration like 90d or an absolute YYYY-MM-DD date (default 90d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// tagDays returns the set of days where any enhanced tag matches one of the
// given tag strings (case-insensitive substring match against tag_type_code
// or comment).
func tagDays(db *store.Store, tags []string, start, end string) (map[string]bool, error) {
	rows, err := db.DB().Query(
		`SELECT day, tag_type_code, comment FROM enhanced_tag WHERE day >= ? AND day <= ? AND day IS NOT NULL`,
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lowered := make([]string, len(tags))
	for i, t := range tags {
		lowered[i] = strings.ToLower(t)
	}

	result := make(map[string]bool)
	for rows.Next() {
		var day, code, comment sql.NullString
		if err := rows.Scan(&day, &code, &comment); err != nil {
			continue
		}
		if !day.Valid {
			continue
		}
		haystack := strings.ToLower(code.String + " " + comment.String)
		for _, t := range lowered {
			if t != "" && strings.Contains(haystack, t) {
				result[day.String] = true
				break
			}
		}
	}
	return result, rows.Err()
}
