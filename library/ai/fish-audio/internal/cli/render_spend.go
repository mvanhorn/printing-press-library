// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aggregate local render spend.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

func newNovelRenderSpendCmd(flags *rootFlags) *cobra.Command {
	var (
		flagGroupBy string
		flagSince   string
		flagDB      string
	)

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Total Fish Audio spend grouped by voice, model, or day for a side-by-side with your ElevenLabs invoice.",
		Long: `Aggregates the local render log into per-group totals, highest spend first.

cost_usd is what was billed; cost_usd_paid_equiv is what the same renders would
have cost at the paid rate. They differ only for s2.1-pro-free, which bills
nothing while still doing the work, so the free tier's real value stays visible
in the report.

Use 'render log' instead when the question is which renders happened rather
than what they added up to.`,
		Example: strings.Trim(`
  fish-audio-pp-cli render spend --group-by model
  fish-audio-pp-cli render spend --group-by voice --since 30d --json
  fish-audio-pp-cli render spend --group-by day --since 2026-08-01 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "render spend")
			}
			groupBy := strings.TrimSpace(flagGroupBy)
			if groupBy == "" {
				groupBy = "model"
			}
			valid := false
			for _, g := range store.RenderSpendGroups() {
				if g == groupBy {
					valid = true
					break
				}
			}
			if !valid {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid value %q for --group-by: must be one of %s", flagGroupBy, strings.Join(store.RenderSpendGroups(), ", ")))
			}
			since, err := fishSince(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			dbPath := fishRenderDBPath(flagDB)
			if stop, mirrorErr := fishMissingMirror(cmd, flags, dbPath, make([]store.SpendRow, 0)); stop {
				return mirrorErr
			}
			db, err := openRenderStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.RenderSpend(cmd.Context(), groupBy, since)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No spend recorded in this window.")
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "%s\tRENDERS\tBYTES IN\tCOST\tPAID EQUIVALENT\n", strings.ToUpper(groupBy))
			var totalCost, totalPaid float64
			var totalRenders, totalBytes int64
			for _, row := range rows {
				fmt.Fprintf(w, "%s\t%d\t%d\t$%.6f\t$%.6f\n", row.Group, row.Renders, row.BytesIn, row.CostUSD, row.CostUSDPaidEquiv)
				totalCost += row.CostUSD
				totalPaid += row.CostUSDPaidEquiv
				totalRenders += row.Renders
				totalBytes += row.BytesIn
			}
			fmt.Fprintf(w, "TOTAL\t%d\t%d\t$%.6f\t$%.6f\n", totalRenders, totalBytes, totalCost, totalPaid)
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "model", "Aggregation dimension (one of: voice, model, day)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Count only renders newer than this duration (30d, 12h) or date (2026-08-01)")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
