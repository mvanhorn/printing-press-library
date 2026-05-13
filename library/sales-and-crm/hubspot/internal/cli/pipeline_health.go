// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local pipeline health analysis command.
func newPipelineHealthCmd(flags *rootFlags) *cobra.Command {
	var flagOwner string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "pipeline-health",
		Short: "Rank synced deals by weighted pipeline value.",
		Long:  `Reads local deals and pipeline stages to compute weighted value and approximate days in stage from hs_lastmodifieddate.`,
		Example: `  hubspot-pp-cli pipeline-health --agent
  hubspot-pp-cli pipeline-health --owner 123 --limit 25 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			result, err := computePipelineHealth(cmd.Context(), db, flagOwner, flagLimit)
			if err != nil {
				return err
			}
			result.TotalWeightedValue = round2(result.TotalWeightedValue)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "dealId\tdealname\tstage\tamount\tprobability\tweighted_value\tage_in_stage_days")
			for _, r := range result.Rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%.2f\t%.2f\t%d\n", r.DealID, r.DealName, r.Stage, r.Amount, r.Probability, r.WeightedValue, r.AgeInStageDays)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagOwner, "owner", "", "HubSpot owner ID to filter by")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max rows to return")
	return cmd
}
