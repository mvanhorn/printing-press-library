// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call — calls the hand-written sibling client (internal/niftyindices or internal/indiapassivefunds) via a package-local newXClient() helper, not the generated internal/client package.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type trackingFidelityRow struct {
	SchemeID      string  `json:"scheme_id"`
	SchemeName    string  `json:"scheme_name"`
	ExpenseRatio  float64 `json:"expense_ratio"`
	TrackingError float64 `json:"tracking_error"`
	TrackingDiff  float64 `json:"tracking_difference"`
	AUM           string  `json:"aum,omitempty"`
	RatiosAsOf    string  `json:"ratios_as_of,omitempty"`
}

func newNovelIndexTrackingCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:         "tracking <index>",
		Short:       "Rank every fund tracking an index by cost and NAV fidelity against the index level.",
		Long:        "Use for a ranked table of all funds tracking an index by fidelity/cost. Use 'index funds' for a plain membership list with no fidelity math, and 'compare' for one fund against one index.",
		Example:     "  passive-indices-pp-cli index tracking \"NIFTY 50\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank tracking funds by cost and fidelity")
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
			if limit > 0 && len(trackers) > limit {
				trackers = trackers[:limit]
			}

			rows := make([]trackingFidelityRow, 0, len(trackers))
			var fetchFailures []map[string]string
			for _, t := range trackers {
				if t.SchemeID == "" {
					continue
				}
				fd, err := c.FundDetail(ctx, t.SchemeID)
				if err != nil {
					fetchFailures = append(fetchFailures, map[string]string{"scheme_id": t.SchemeID, "error": err.Error()})
					continue
				}
				rows = append(rows, trackingFidelityRow{
					SchemeID:      fd.SchemeID,
					SchemeName:    fd.SchemeName,
					ExpenseRatio:  fd.ExpenseRatio,
					TrackingError: fd.TrackingError,
					TrackingDiff:  fd.TrackingDiff,
					AUM:           fd.AUM,
					RatiosAsOf:    fd.RatiosAsOf,
				})
			}
			// A 0 expense ratio means "not yet disclosed", not "free" — sort
			// those to the end so an undisclosed fund never appears to be
			// the cheapest by default.
			sort.Slice(rows, func(i, j int) bool {
				if (rows[i].ExpenseRatio <= 0) != (rows[j].ExpenseRatio <= 0) {
					return rows[j].ExpenseRatio <= 0
				}
				return rows[i].ExpenseRatio < rows[j].ExpenseRatio
			})

			if len(fetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; ranking computed over the remaining %d funds\n", len(fetchFailures), len(trackers), len(rows))
			}
			out := map[string]any{
				"index":         args[0],
				"matched_index": matched,
				"ranked_funds":  rows,
			}
			if len(fetchFailures) > 0 {
				out["fetch_failures"] = fetchFailures
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum tracking funds to rank (bounds fan-out fetch cost)")
	return cmd
}
