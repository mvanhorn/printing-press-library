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

// newNovelRotationCmd ranks sectors by movement over a window and attributes
// each move to its largest constituents. The portal's sector feed is
// volume-only and point-in-time, so both the window and the attribution
// require local history.
func newNovelRotationCmd(flags *rootFlags) *cobra.Command {
	var window, dbPath string
	var top, contributors int
	cmd := &cobra.Command{
		Use:   "rotation",
		Short: "Rank sectors by movement over a window and name the constituents that drove each move.",
		Long: "Use this command to rank sectors by movement over a window and see which constituents drove each one.\n" +
			"Do NOT use it for the current top-10-by-volume snapshot; use 'sectors top' instead.\n" +
			"Do NOT use it to list a single sector's members; use 'sectors summary' instead.",
		Example:     "  psx-pp-cli rotation --window 30d --top 5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rotation")
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
			type contributor struct {
				Symbol   string  `json:"symbol"`
				DeltaPct float64 `json:"delta_pct"`
			}
			type sectorMove struct {
				Sector       string        `json:"sector"`
				Members      int           `json:"members"`
				MeanDeltaPct float64       `json:"mean_delta_pct"`
				Contributors []contributor `json:"contributors"`
			}
			view := struct {
				From    string       `json:"from"`
				To      string       `json:"to"`
				Count   int          `json:"count"`
				Sectors []sectorMove `json:"sectors"`
				Note    string       `json:"note,omitempty"`
			}{Sectors: make([]sectorMove, 0)}

			if len(times) < 2 {
				view.Note = fmt.Sprintf("only %d market snapshot(s) stored; rotation needs two. Run 'psx-pp-cli snapshot take' again later.", len(times))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			newest := times[0]
			oldest := times[len(times)-1]
			if strings.TrimSpace(window) != "" {
				d, err := cliutil.ParseDurationLoose(window)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--window %q is not a duration (try 30d, 4w): %w", window, err))
				}
				cutoff := nowUTC().Add(-d).Format(snapshotTimeFormat)
				for _, t := range times {
					if t <= cutoff {
						oldest = t
						break
					}
				}
			}
			if oldest == newest {
				oldest = times[1]
				view.Note = "no snapshot older than the requested window; comparing the two most recent instead"
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

			bySector := map[string][]contributor{}
			for sym, cRow := range curr {
				pRow, ok := prev[sym]
				if !ok {
					continue
				}
				cp, okc := parseNum(cRow["current"])
				pp, okp := parseNum(pRow["current"])
				if !okc || !okp || pp == 0 {
					continue
				}
				sector := strings.TrimSpace(cRow["sector"])
				if sector == "" {
					sector = "(unclassified)"
				}
				bySector[sector] = append(bySector[sector], contributor{Symbol: sym, DeltaPct: (cp - pp) / pp * 100})
			}
			for sector, members := range bySector {
				if len(members) == 0 {
					continue
				}
				sum := 0.0
				for _, m := range members {
					sum += m.DeltaPct
				}
				sort.Slice(members, func(i, j int) bool {
					return absf(members[i].DeltaPct) > absf(members[j].DeltaPct)
				})
				keep := members
				if contributors > 0 && len(keep) > contributors {
					keep = keep[:contributors]
				}
				view.Sectors = append(view.Sectors, sectorMove{
					Sector:       sector,
					Members:      len(members),
					MeanDeltaPct: sum / float64(len(members)),
					Contributors: keep,
				})
			}
			sort.Slice(view.Sectors, func(i, j int) bool {
				return view.Sectors[i].MeanDeltaPct > view.Sectors[j].MeanDeltaPct
			})
			// Show the leading and lagging ends. Guard the overlap: with fewer
			// than 2*top sectors the two slices intersect and would emit the
			// middle ones twice, inflating Count and double-counting any
			// aggregate an agent computes.
			if top > 0 && len(view.Sectors) > 2*top {
				head := append([]sectorMove{}, view.Sectors[:top]...)
				tail := view.Sectors[len(view.Sectors)-top:]
				view.Sectors = append(head, tail...)
			}
			view.Count = len(view.Sectors)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No sectors comparable across the two snapshots.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n\n", view.From, view.To)
			for _, sm := range view.Sectors {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %7.2f%%  (%d members)\n", cliutil.ScrubTerminal(sm.Sector), sm.MeanDeltaPct, sm.Members)
				for _, c := range sm.Contributors {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-12s %7.2f%%\n", cliutil.ScrubTerminal(c.Symbol), c.DeltaPct)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "comparison window (30d, 4w)")
	cmd.Flags().IntVar(&top, "top", 5, "leading and lagging sectors to show (0 = all)")
	cmd.Flags().IntVar(&contributors, "contributors", 3, "constituents to name per sector")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}
