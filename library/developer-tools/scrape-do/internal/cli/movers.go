// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command `movers` — scan every tracked query's two most
// recent SERP snapshots and surface only the queries whose top positions moved
// past a threshold. A local aggregation across all stored snapshots no single
// API call can produce. Offline, no credits. Hand file (no generator header).

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// queryMover summarizes the largest movement within one tracked query.
type queryMover struct {
	Query     string `json:"query"`
	ParamHash string `json:"param_hash"`
	TopDomain string `json:"top_domain"`
	Delta     int    `json:"delta"`     // signed position change of the top mover (+ = up)
	Status    string `json:"status"`    // moved | new | dropped
	MaxMove   int    `json:"max_move"`  // absolute magnitude used for the threshold
	Snapshots int    `json:"snapshots"` // total snapshots stored for this query
}

func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var threshold int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Across all tracked queries, list only the ones whose top SERP positions moved — offline",
		Long: `Scan every locally-tracked query (those with at least two stored SERP
snapshots), diff each query's two most recent snapshots, and list only the
queries whose largest rank movement meets the --threshold. Turns hundreds of
tracked keywords into a short 'what changed' list. Offline — no API call, no
credits. Seed it by running 'google search' on your keywords over time.`,
		Example:     "  scrape-do-pp-cli movers --threshold 3\n  scrape-do-pp-cli movers --threshold 5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("movers takes no arguments (use --threshold to filter)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if threshold < 1 {
				threshold = 1
			}
			st, ext, err := openExtras(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			tracked, err := ext.TrackedHashes(cmd.Context())
			if err != nil {
				return err
			}
			out := make([]queryMover, 0, len(tracked))
			for _, t := range tracked {
				cur, prev, err := ext.TwoLatestSnapshots(cmd.Context(), t.ParamHash)
				if err != nil || cur == nil || prev == nil {
					continue
				}
				curOrg, _ := ext.OrganicForSnapshot(cmd.Context(), cur.ID)
				prevOrg, _ := ext.OrganicForSnapshot(cmd.Context(), prev.ID)
				movers := diffOrganic(curOrg, prevOrg)
				best := topMover(movers)
				if best == nil {
					continue
				}
				mag := absDelta(*best)
				if mag < threshold {
					continue
				}
				qm := queryMover{
					Query: t.Query, ParamHash: t.ParamHash, TopDomain: best.Domain,
					Status: best.Status, MaxMove: mag, Snapshots: t.Count,
				}
				if best.Delta != nil {
					qm.Delta = *best.Delta
				}
				out = append(out, qm)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].MaxMove > out[j].MaxMove })

			payload := map[string]any{"threshold": threshold, "tracked_queries": len(tracked), "movers": out}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no queries moved by >= %d positions across %d tracked queries\n", threshold, len(tracked))
				return nil
			}
			rows := make([][]string, 0, len(out))
			for _, m := range out {
				rows = append(rows, []string{m.Query, m.TopDomain, deltaStr(&m.Delta), m.Status, strconv.Itoa(m.MaxMove)})
			}
			return flags.printTable(cmd, []string{"QUERY", "TOP MOVER", "DELTA", "STATUS", "MOVE"}, rows)
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 3, "Minimum absolute position movement to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// topMover returns the largest-magnitude mover from a diff (already sorted by
// diffOrganic), or nil when there are none.
func topMover(movers []driftMover) *driftMover {
	for i := range movers {
		if movers[i].Status != "unchanged" {
			return &movers[i]
		}
	}
	return nil
}
