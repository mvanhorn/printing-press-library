// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
)

// newNovelDiffCmd answers "what changed since last time". The portal serves a
// current view only, so this reads two locally retained snapshots.
func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	var watchOnly bool
	var top int

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show what changed in price, volume and valuation metrics between two synced snapshots.",
		Long: "Use this command to see what changed between two points in time across the market or your watchlist.\n" +
			"Do NOT use it for the current-state market table; use 'market watch' instead.\n" +
			"Do NOT use it for a full OHLCV series of one symbol; use 'history' instead.\n" +
			"Requires at least two captures from 'snapshot take'.",
		Example:     "  psx-pp-cli diff --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if mirrorMissing(dbPath) {
				return writeMirrorHint(cmd, flags, orDefaultDB(dbPath), "snapshot")
			}
			s, _, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			times, err := listSnapshotTimes(ctx, s, snapshotKindMarket)
			if err != nil {
				return err
			}
			type change struct {
				Symbol     string  `json:"symbol"`
				FromPrice  float64 `json:"from_price"`
				ToPrice    float64 `json:"to_price"`
				DeltaPct   float64 `json:"delta_pct"`
				FromVolume float64 `json:"from_volume,omitempty"`
				ToVolume   float64 `json:"to_volume,omitempty"`
			}
			view := struct {
				From    string   `json:"from"`
				To      string   `json:"to"`
				Count   int      `json:"count"`
				Changes []change `json:"changes"`
				Note    string   `json:"note,omitempty"`
			}{Changes: make([]change, 0)}

			if len(times) < 2 {
				view.Note = fmt.Sprintf("only %d market snapshot(s) stored; diff needs two. Run 'psx-pp-cli snapshot take' again later.", len(times))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			newest := times[0]
			oldest := times[len(times)-1]
			if strings.TrimSpace(since) != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is not a duration (try 7d, 24h, 2w): %w", since, err))
				}
				cutoff := nowUTC().Add(-d).Format(snapshotTimeFormat)
				oldest = times[len(times)-1]
				for _, t := range times {
					if t <= cutoff {
						oldest = t
						break
					}
				}
			}
			if oldest == newest {
				view.Note = "no snapshot older than the requested window; comparing the two most recent instead"
				oldest = times[1]
			}
			view.From, view.To = oldest, newest

			prev, err := loadSnapshot(ctx, s, snapshotKindMarket, oldest)
			if err != nil {
				return err
			}
			curr, err := loadSnapshot(ctx, s, snapshotKindMarket, newest)
			if err != nil {
				return err
			}
			var only map[string]bool
			if watchOnly {
				syms, _, err := watchlistSymbols(ctx, s)
				if err != nil {
					return err
				}
				only = map[string]bool{}
				for _, sym := range syms {
					only[sym] = true
				}
				if len(only) == 0 {
					view.Note = "watchlist is empty; add names with 'psx-pp-cli watchlist track OGDC'"
				}
			}

			for sym, cRow := range curr {
				if only != nil && !only[sym] {
					continue
				}
				pRow, ok := prev[sym]
				if !ok {
					continue
				}
				cp, okc := parseNum(cRow["current"])
				pp, okp := parseNum(pRow["current"])
				if !okc || !okp || pp == 0 {
					continue
				}
				ch := change{Symbol: sym, FromPrice: pp, ToPrice: cp, DeltaPct: (cp - pp) / pp * 100}
				if v, ok := parseNum(pRow["volume"]); ok {
					ch.FromVolume = v
				}
				if v, ok := parseNum(cRow["volume"]); ok {
					ch.ToVolume = v
				}
				view.Changes = append(view.Changes, ch)
			}
			sort.Slice(view.Changes, func(i, j int) bool {
				return absf(view.Changes[i].DeltaPct) > absf(view.Changes[j].DeltaPct)
			})
			if top > 0 && len(view.Changes) > top {
				view.Changes = view.Changes[:top]
			}
			view.Count = len(view.Changes)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No comparable symbols across the two snapshots.")
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n\n", view.From, view.To)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s %10s %9s\n", "SYMBOL", "FROM", "TO", "DELTA%")
			for _, c := range view.Changes {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f %10.2f %8.2f%%\n", c.Symbol, c.FromPrice, c.ToPrice, c.DeltaPct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "compare against the newest snapshot at least this old (7d, 24h, 2w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().BoolVar(&watchOnly, "watchlist", false, "restrict to symbols on the watchlist")
	cmd.Flags().IntVar(&top, "top", 25, "maximum rows to return, ranked by absolute move (0 = all)")
	return cmd
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
