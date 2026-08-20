// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type seasonalityResult struct {
	Keyword          string             `json:"keyword"`
	Geo              string             `json:"geo"`
	MonthlyAverages  map[string]float64 `json:"monthly_averages"`
	PeakMonth        int                `json:"peak_month"`
	SeasonalityScore float64            `json:"seasonality_score"`
	DataPointsUsed   int                `json:"data_points_used"`
	Note             string             `json:"note,omitempty"`
}

// minSeasonalityDataPoints is the rough threshold below which a monthly
// average is more noise than signal (roughly under 2.5 years of monthly
// coverage). Below it, the result still ships but with a reliability note.
const minSeasonalityDataPoints = 30

// computeSeasonality groups interest points by calendar month (1-12) across
// however many years of history exist, averages each month, and derives a
// simple coefficient-of-variation "seasonality strength" score
// (stddev/mean of the monthly averages) — higher means more seasonal swing.
func computeSeasonality(keyword, geo string, rows []gtInterestPointRecord) seasonalityResult {
	sums := map[int]float64{}
	counts := map[int]int{}
	for _, r := range rows {
		d, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}
		m := int(d.Month())
		sums[m] += float64(r.Value)
		counts[m]++
	}

	averages := make(map[string]float64, len(sums))
	values := make([]float64, 0, 12)
	peakMonth := 0
	peakAvg := -1.0
	for m := 1; m <= 12; m++ {
		if counts[m] == 0 {
			continue
		}
		avg := sums[m] / float64(counts[m])
		averages[strconv.Itoa(m)] = avg
		values = append(values, avg)
		if avg > peakAvg {
			peakAvg = avg
			peakMonth = m
		}
	}

	var score float64
	if len(values) > 0 {
		mean := 0.0
		for _, v := range values {
			mean += v
		}
		mean /= float64(len(values))
		if mean != 0 {
			variance := 0.0
			for _, v := range values {
				variance += (v - mean) * (v - mean)
			}
			variance /= float64(len(values))
			score = math.Sqrt(variance) / mean
		}
	}

	result := seasonalityResult{
		Keyword:          keyword,
		Geo:              geo,
		MonthlyAverages:  averages,
		PeakMonth:        peakMonth,
		SeasonalityScore: score,
		DataPointsUsed:   len(rows),
	}
	if len(rows) < minSeasonalityDataPoints {
		result.Note = fmt.Sprintf("fewer than %d data points available (%d found); seasonality estimate may be unreliable", minSeasonalityDataPoints, len(rows))
	}
	return result
}

// pp:data-source local
func newNovelTrendsSeasonalityCmd(flags *rootFlags) *cobra.Command {
	var flagGeo string

	cmd := &cobra.Command{
		Use:         "seasonality <keyword>",
		Short:       "Computes monthly averages, peak month, and a seasonality strength score for a keyword from cached history.",
		Example:     "  google-trends-pp-cli trends seasonality \"pumpkin spice\" --geo US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("keyword argument is required"))
			}
			keyword := args[0]

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "google-trends-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				empty := seasonalityResult{Keyword: keyword, Geo: flagGeo, MonthlyAverages: map[string]float64{}}
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends interest "+keyword, empty)
			}
			defer db.Close()

			rows, err := queryInterestPointsForKeywordGeo(db, keyword, flagGeo)
			if err != nil {
				return fmt.Errorf("querying interest points: %w", err)
			}
			rows = filterInterestRowsToLatestScope(rows)
			if len(rows) == 0 {
				return notFoundErr(fmt.Errorf("no interest-over-time history in the local store for %q; run 'trends interest %s' first", keyword, keyword))
			}

			return printLocalResult(cmd, flags, computeSeasonality(keyword, flagGeo, rows))
		},
	}
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Geo code to scope the seasonality calculation (e.g. US); default: all cached geos")
	return cmd
}
