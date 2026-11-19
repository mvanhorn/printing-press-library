// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type mktTermStat struct {
	Term      string  `json:"term"`
	TopCount  int     `json:"top_count"`
	TopRate   float64 `json:"top_rate"`
	RestCount int     `json:"rest_count"`
	RestRate  float64 `json:"rest_rate"`
	Diff      float64 `json:"diff"`
}

// mktSplitQuartile ranks listings best-first and returns the top quartile
// and the rest. Demand is used when any listing has review data, otherwise
// position decides.
func mktSplitQuartile(uniq []mktRow, dem map[int64]mktListingDemand) (top, rest []mktRow, basis string) {
	haveDemand := false
	for _, r := range uniq {
		if d, ok := dem[r.ListingID]; ok && d.Collected {
			haveDemand = true
			break
		}
	}
	sorted := append([]mktRow(nil), uniq...)
	if haveDemand {
		basis = "demand"
		score := func(r mktRow) int {
			if d, ok := dem[r.ListingID]; ok && d.Collected {
				return d.Reviews
			}
			return -1 // no review data ranks below any measured listing
		}
		sort.SliceStable(sorted, func(i, j int) bool {
			si, sj := score(sorted[i]), score(sorted[j])
			if si != sj {
				return si > sj
			}
			return sorted[i].Position < sorted[j].Position
		})
	} else {
		basis = "position"
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	}
	n := (len(sorted) + 3) / 4
	return sorted[:n], sorted[n:], basis
}

func mktContrast(top, rest []mktRow, terms func(mktRow) []string, minSupport, limit int) []mktTermStat {
	tc, rc := map[string]int{}, map[string]int{}
	for _, r := range top {
		for _, t := range terms(r) {
			tc[t]++
		}
	}
	for _, r := range rest {
		for _, t := range terms(r) {
			rc[t]++
		}
	}
	all := map[string]bool{}
	for t := range tc {
		all[t] = true
	}
	for t := range rc {
		all[t] = true
	}
	out := make([]mktTermStat, 0)
	for t := range all {
		if tc[t]+rc[t] < minSupport {
			continue
		}
		s := mktTermStat{Term: t, TopCount: tc[t], RestCount: rc[t]}
		if len(top) > 0 {
			s.TopRate = mktRound(float64(tc[t])/float64(len(top)), 3)
		}
		if len(rest) > 0 {
			s.RestRate = mktRound(float64(rc[t])/float64(len(rest)), 3)
		}
		s.Diff = mktRound(s.TopRate-s.RestRate, 3)
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Diff != out[j].Diff {
			return out[i].Diff > out[j].Diff
		}
		if out[i].TopCount != out[j].TopCount {
			return out[i].TopCount > out[j].TopCount
		}
		return out[i].Term < out[j].Term
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func newNovelMarketTagsCmd(flags *rootFlags) *cobra.Command {
	var flagMinSupport, flagLimit int
	var flagSnapshot int64
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Contrast the tags and title words of the highest-demand listings against the rest.",
		Long: `Contrast the tags and title words of the highest-demand listings against the rest.

Listings are split into the top-demand quartile and everything else, using review demand when the
snapshot has review data and result position otherwise. For each tag and title word the output gives
how many listings in each group use it, the share, and the difference. Words seen in fewer than
--min-support listings overall are left out.
--csv and --plain are refused for this nested report; use --json or --agent.`,
		Example:     "  etsy-pp-cli market tags --min-support 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mktValidateSnapshotFlag(cmd, flagSnapshot); err != nil {
				return err
			}
			if flags.csv || flags.plain {
				return usageErr(fmt.Errorf("market tags output is nested; use --json"))
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market tags")
			}
			window, err := mktParseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("--window: %w", err))
			}
			if flagMinSupport < 1 {
				return usageErr(fmt.Errorf("--min-support must be at least 1"))
			}
			db, err := mktOpenRead(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db != nil {
				defer db.Close()
			}
			snap, note, err := mktPickSnapshot(db, flagSnapshot)
			if err != nil {
				return err
			}
			if snap == nil {
				return mktEmitReport(cmd, flags, map[string]any{"results": make([]any, 0), "note": note})
			}
			rows, err := mktLoadResults(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			dem, err := mktLoadDemand(db.DB(), snap, window)
			if err != nil {
				return err
			}
			uniq := mktBestRows(rows)
			if len(uniq) == 0 {
				return mktEmitReport(cmd, flags, map[string]any{"snapshot_id": snap.ID, "results": make([]any, 0), "note": "snapshot has no listings"})
			}
			top, rest, basis := mktSplitQuartile(uniq, dem)
			tags := mktContrast(top, rest, func(r mktRow) []string {
				seen := map[string]bool{}
				out := make([]string, 0)
				for _, t := range r.Tags {
					if t = strings.ToLower(strings.TrimSpace(t)); t != "" && !seen[t] {
						seen[t] = true
						out = append(out, t)
					}
				}
				return out
			}, flagMinSupport, flagLimit)
			words := mktContrast(top, rest, func(r mktRow) []string { return mktTitleWords(r.Title) }, flagMinSupport, flagLimit)
			return mktEmitReport(cmd, flags, map[string]any{
				"snapshot_id": snap.ID, "results": map[string]any{"basis": basis, "min_support": flagMinSupport,
					"top_group_size": len(top), "rest_group_size": len(rest),
					"tags": tags, "title_words": words},
			})
		},
	}
	cmd.Flags().IntVar(&flagMinSupport, "min-support", 3, "Minimum listings (both groups combined) a tag or word must appear in")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max tags and title words to list (0 = all)")
	cmd.Flags().StringVar(&flagWindow, "window", "12mo", "Review window used when ranking by demand")
	cmd.Flags().Int64Var(&flagSnapshot, "snapshot", 0, mktSnapshotUsage)
	mktHideZeroDefault(cmd, "snapshot")
	return cmd
}
