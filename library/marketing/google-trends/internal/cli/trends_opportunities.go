// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type opportunityResult struct {
	Term        string  `json:"term"`
	RisingValue int     `json:"rising_value"`
	IsBreakout  bool    `json:"is_breakout"`
	Score       float64 `json:"score"`
}

// latestRisingTerms filters rows to kind=="rising" and keeps only those from
// the most recent sync instant, so stale rising terms from earlier syncs
// don't pile up alongside the current snapshot.
func latestRisingTerms(rows []gtRelatedTermRecord) []gtRelatedTermRecord {
	rising := make([]gtRelatedTermRecord, 0, len(rows))
	syncedAtValues := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Kind == "rising" {
			rising = append(rising, r)
			syncedAtValues = append(syncedAtValues, r.SyncedAt)
		}
	}
	times := distinctSyncedAtDesc(syncedAtValues)
	if len(times) == 0 {
		return rising
	}
	latest := times[0]
	out := make([]gtRelatedTermRecord, 0, len(rising))
	for _, r := range rising {
		if t, err := time.Parse(time.RFC3339, r.SyncedAt); err == nil && t.Equal(latest) {
			out = append(out, r)
		}
	}
	return out
}

// computeTrendSlope estimates a simple normalized growth rate for the
// keyword's interest-over-time series: (most recent value - oldest value)
// divided by the value range observed across the series. This is
// deliberately a ratio, not a fitted regression — simplest formula that
// still rewards a keyword whose interest is climbing relative to its own
// historical volatility. Returns 0 when there isn't enough data, or when
// every observed value is identical (no range to normalize by).
func computeTrendSlope(rows []gtInterestPointRecord) float64 {
	if len(rows) < 2 {
		return 0
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
	oldest := rows[0].Value
	newest := rows[len(rows)-1].Value
	minV, maxV := rows[0].Value, rows[0].Value
	for _, r := range rows {
		if r.Value < minV {
			minV = r.Value
		}
		if r.Value > maxV {
			maxV = r.Value
		}
	}
	valueRange := maxV - minV
	if valueRange == 0 {
		return 0
	}
	return float64(newest-oldest) / float64(valueRange)
}

// pp:data-source local
func newNovelTrendsOpportunitiesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "opportunities <keyword>",
		Short:       "Ranks a keyword's related/rising terms by rising-momentum times parent-topic growth",
		Example:     "  google-trends-pp-cli trends opportunities \"meal prep\" --agent",
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
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends related "+keyword, make([]opportunityResult, 0))
			}
			defer db.Close()

			relatedRows, err := queryRelatedTermsForKeyword(db, keyword)
			if err != nil {
				return fmt.Errorf("querying related terms: %w", err)
			}
			relatedRows = filterRelatedRowsToLatestScope(relatedRows)
			interestRows, err := queryInterestPointsForKeyword(db, keyword)
			if err != nil {
				return fmt.Errorf("querying interest points: %w", err)
			}
			interestRows = filterInterestRowsToLatestScope(interestRows)

			slope := computeTrendSlope(interestRows)
			rising := latestRisingTerms(relatedRows)

			results := make([]opportunityResult, 0, len(rising))
			for _, r := range rising {
				score := float64(r.Value) * (1 + math.Max(0, slope))
				results = append(results, opportunityResult{Term: r.Term, RisingValue: r.Value, IsBreakout: r.IsBreakout, Score: score})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}

			if len(results) == 0 {
				return notFoundErr(fmt.Errorf("no rising related terms found in the local store for %q; run 'trends related %s' first", keyword, keyword))
			}
			return printLocalResult(cmd, flags, results)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of opportunities to return")
	return cmd
}
