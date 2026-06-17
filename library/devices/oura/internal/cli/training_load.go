// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type trainingLoadRow struct {
	Day                 string  `json:"day"`
	RollingLoad7d       float64 `json:"rolling_load_7d"`
	NextDayReadiness    float64 `json:"next_day_readiness,omitempty"`
	HasNextDayReadiness bool    `json:"has_next_day_readiness"`
}

type trainingLoadView struct {
	Days        int               `json:"days"`
	Rows        []trainingLoadRow `json:"rows"`
	Correlation float64           `json:"load_readiness_correlation,omitempty"`
	Verdict     string            `json:"verdict"`
	Note        string            `json:"note,omitempty"`
}

func newNovelTrainingLoadCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "training-load",
		Short: "See your accumulated training stress (7-day rolling)",
		Long: `Shows your 7-day rolling training load (summed workout calories) alongside
next-day readiness scores with a 1-day lag, to reveal the recovery debt
curve before it peaks. Reports the correlation between rolling load and
next-day readiness over the window.`,
		Example:     `  oura-pp-cli training-load --since 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute 7-day rolling training load vs next-day readiness")
				return nil
			}
			start, err := resolveSinceDay(flagSince, 30)
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
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			loadByDay, err := dailyCalories(db, addDays(start, -7), end)
			if err != nil {
				return fmt.Errorf("querying workout calories: %w", err)
			}
			readinessSpec, _ := resolveMetric("readiness")
			readinessByDay, err := metricSeries(db, readinessSpec, start, addDays(end, 1))
			if err != nil {
				return fmt.Errorf("querying readiness: %w", err)
			}

			var rows []trainingLoadRow
			var loadSamples, readinessSamples []float64
			for d := start; d <= end; d = addDays(d, 1) {
				load := rollingSum(loadByDay, addDays(d, -6), d)
				row := trainingLoadRow{Day: d, RollingLoad7d: round2(load)}
				if r, ok := readinessByDay[addDays(d, 1)]; ok {
					row.NextDayReadiness = r
					row.HasNextDayReadiness = true
					loadSamples = append(loadSamples, load)
					readinessSamples = append(readinessSamples, r)
				}
				rows = append(rows, row)
			}

			view := trainingLoadView{Days: daysBetween(start, end) + 1, Rows: rows}
			if len(loadSamples) >= 3 {
				view.Correlation = round2(pearsonCorrelation(loadSamples, readinessSamples))
				switch {
				case view.Correlation < -0.3:
					view.Verdict = fmt.Sprintf("higher training load tends to precede lower next-day readiness (r=%.2f)", view.Correlation)
				case view.Correlation > 0.3:
					view.Verdict = fmt.Sprintf("higher training load tends to precede higher next-day readiness (r=%.2f)", view.Correlation)
				default:
					view.Verdict = fmt.Sprintf("no strong relationship between training load and next-day readiness in this window (r=%.2f)", view.Correlation)
				}
			} else {
				view.Verdict = "insufficient-data"
				view.Note = "need at least 3 days with both workout and next-day readiness data to estimate a correlation"
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "day\trolling_load_7d\tnext_day_readiness")
			for _, r := range rows {
				if r.HasNextDayReadiness {
					fmt.Fprintf(out, "%s\t%.0f\t%.0f\n", r.Day, r.RollingLoad7d, r.NextDayReadiness)
				} else {
					fmt.Fprintf(out, "%s\t%.0f\t-\n", r.Day, r.RollingLoad7d)
				}
			}
			fmt.Fprintln(out, "\n"+view.Verdict)
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Start of the window: a duration like 30d or an absolute YYYY-MM-DD date (default 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func dailyCalories(db *store.Store, start, end string) (map[string]float64, error) {
	rows, err := db.DB().Query(
		`SELECT day, SUM(calories) FROM workout WHERE day >= ? AND day <= ? AND day IS NOT NULL GROUP BY day`,
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]float64)
	for rows.Next() {
		var day sql.NullString
		var cal sql.NullFloat64
		if err := rows.Scan(&day, &cal); err != nil {
			continue
		}
		if day.Valid {
			result[day.String] = cal.Float64
		}
	}
	return result, rows.Err()
}

func rollingSum(byDay map[string]float64, start, end string) float64 {
	var sum float64
	for d := start; d <= end; d = addDays(d, 1) {
		sum += byDay[d]
	}
	return sum
}

func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) {
		return 0
	}
	meanX, _ := meanStdDev(x)
	meanY, _ := meanStdDev(y)
	var num, denX, denY float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	if denX == 0 || denY == 0 {
		return 0
	}
	return num / math.Sqrt(denX*denY)
}
