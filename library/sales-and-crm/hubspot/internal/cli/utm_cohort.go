// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local UTM campaign cohort rollup command.
func newUtmCohortCmd(flags *rootFlags) *cobra.Command {
	var flagCampaign string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "utm-cohort [campaign]",
		Short: "Roll up synced contacts and deals by UTM campaign.",
		Long:  `Builds a local cohort for one utm_campaign and aggregates associated deals by pipeline stage.`,
		Example: `  hubspot-pp-cli utm-cohort spring-launch --agent
  hubspot-pp-cli utm-cohort --campaign spring-launch --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			campaign := flagCampaign
			if campaign == "" && len(args) > 0 {
				campaign = args[0]
			}
			if campaign == "" {
				return usageErr(fmt.Errorf("campaign is required as argument or --campaign"))
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			result, err := computeUtmCohort(cmd.Context(), db, campaign, flagLimit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "campaign\tcohort_size\ttotal_deals\tpercent_closed_won\tavg_deal_amount")
			fmt.Fprintf(w, "%s\t%d\t%d\t%.2f\t%.2f\n", result.Campaign, result.CohortSize, result.TotalDeals, result.PercentClosedWon, result.AvgDealAmount)
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagCampaign, "campaign", "", "UTM campaign name")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max stage rows to return")
	return cmd
}
