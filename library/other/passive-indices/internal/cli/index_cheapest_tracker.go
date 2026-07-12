// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call — calls the hand-written sibling client (internal/niftyindices or internal/indiapassivefunds) via a package-local newXClient() helper, not the generated internal/client package.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelIndexCheapestTrackerCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:         "cheapest-tracker <index>",
		Short:       "Find the lowest-cost fund tracking a given index.",
		Example:     "  passive-indices-pp-cli index cheapest-tracker \"NIFTY 50\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find cheapest tracker")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required, e.g. \"NIFTY 50\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			trackers, matched, err := resolveIndexTrackers(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(trackers) == 0 {
				return flags.printJSON(cmd, map[string]any{"index": args[0], "matched_index": matched, "note": "no tracking funds found"})
			}
			// Bound the fan-out: fetch expense ratios for at most 40 candidates.
			maxScan := len(trackers)
			if maxScan > 40 {
				maxScan = 40
			}

			rows := make([]trackingFidelityRow, 0, maxScan)
			var fetchFailures []map[string]string
			var undisclosed []trackingFidelityRow
			for _, t := range trackers[:maxScan] {
				if t.SchemeID == "" {
					continue
				}
				fd, err := c.FundDetail(ctx, t.SchemeID)
				if err != nil {
					fetchFailures = append(fetchFailures, map[string]string{"scheme_id": t.SchemeID, "error": err.Error()})
					continue
				}
				row := trackingFidelityRow{
					SchemeID:      fd.SchemeID,
					SchemeName:    fd.SchemeName,
					ExpenseRatio:  fd.ExpenseRatio,
					TrackingError: fd.TrackingError,
					TrackingDiff:  fd.TrackingDiff,
					AUM:           fd.AUM,
				}
				// A 0 expense ratio means "not yet disclosed", not "free" —
				// exclude from the cheapest ranking so an undisclosed fund
				// never wins by default. It is still reported separately.
				if fd.ExpenseRatio <= 0 {
					undisclosed = append(undisclosed, row)
					continue
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].ExpenseRatio < rows[j].ExpenseRatio })
			if top <= 0 {
				top = 1
			}
			if top > len(rows) {
				top = len(rows)
			}

			out := map[string]any{
				"index":            args[0],
				"matched_index":    matched,
				"scanned_funds":    maxScan,
				"total_candidates": len(trackers),
				"cheapest":         rows[:top],
			}
			if len(undisclosed) > 0 {
				out["undisclosed_expense_ratio"] = undisclosed
			}
			if len(fetchFailures) > 0 {
				out["fetch_failures"] = fetchFailures
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&top, "top", 1, "how many cheapest trackers to return")
	return cmd
}
