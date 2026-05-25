// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
)

// newCounterPoolCmd cross-joins our champion pool with theirs and ranks
// the full matchup matrix by Mobalytics matchup delta.
func newCounterPoolCmd(flags *rootFlags) *cobra.Command {
	var our, their string
	var minSample int64
	var top int
	cmd := &cobra.Command{
		Use:   "counter-pool",
		Short: "Matrix of our champion pool vs theirs, ranked by matchup delta.",
		Long: `For each champion in --our, fetch /counters and join against the
champions listed in --their. The resulting (our × their) matrix is sorted
descending by matchup delta — the SQL coaches do in their head.`,
		Example:     `  mobalytics-lol-pp-cli counter-pool --our jinx,ezreal,kaisa --their swain,brand,karthus --top 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if our == "" || their == "" {
				return fmt.Errorf("both --our and --their are required (CSV of champion slugs)")
			}
			if dryRunOK(flags) {
				return nil
			}
			ours := splitCSV(our)
			theirs := splitCSV(their)
			theirSet := map[string]bool{}
			for _, t := range theirs {
				theirSet[t] = true
			}
			client := moba.NewClient(flags.timeout)
			type cell struct {
				Our          string  `json:"our"`
				Their        string  `json:"their"`
				Role         string  `json:"role"`
				WinRate      float64 `json:"winRate"`
				MatchupDelta float64 `json:"matchupDelta"`
				Sample       int64   `json:"sample"`
			}
			out := []cell{}
			// Track which our×their cells produced no row so the user can
			// distinguish "no data" from "no rendering". Mobalytics's
			// /counters endpoint returns the most common counters per
			// champion, not the full roster — opponents outside that
			// frequent set drop out silently without this disclosure.
			seenPair := map[string]bool{}
			for _, o := range ours {
				html, err := client.Fetch(moba.ChampionPath(o, "counters"))
				if err != nil {
					return fmt.Errorf("fetch %s: %w", o, err)
				}
				rows := moba.ParseCounters(html, o)
				for _, r := range rows {
					if !theirSet[r.OpponentSlug] {
						continue
					}
					if minSample > 0 && r.Sample < minSample {
						continue
					}
					out = append(out, cell{
						Our: o, Their: r.OpponentSlug, Role: r.Role,
						WinRate: r.WinRate, MatchupDelta: r.MatchupDelta, Sample: r.Sample,
					})
					seenPair[o+"|"+r.OpponentSlug] = true
				}
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].MatchupDelta > out[j].MatchupDelta
			})
			if top > 0 && top < len(out) {
				out = out[:top]
			}
			// Surface missing pairs on stderr so an operator knows the
			// matrix is incomplete by upstream-data shape, not by bug.
			expected := len(ours) * len(theirs)
			missing := []string{}
			for _, o := range ours {
				for _, t := range theirs {
					if !seenPair[o+"|"+t] {
						missing = append(missing, o+" vs "+t)
					}
				}
			}
			if len(missing) > 0 {
				preview := missing
				if len(preview) > 6 {
					preview = preview[:6]
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "counter-pool: %d/%d pairings returned no row (missing from Mobalytics frequent-counters list or below --min-sample). Examples: %v\n", expected-len(seenPair), expected, preview)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&our, "our", "", "CSV of our champion slugs (required).")
	cmd.Flags().StringVar(&their, "their", "", "CSV of enemy champion slugs (required).")
	cmd.Flags().Int64Var(&minSample, "min-sample", 0, "Drop matchups with fewer than N games.")
	cmd.Flags().IntVar(&top, "top", 0, "Limit to top-N cells (0 = all).")
	return cmd
}
