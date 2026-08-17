// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFleetReportCmd(flags *rootFlags) *cobra.Command {
	var flagOwner string
	var flagAircraft bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Turn an owner name into a full fleet profile: count, model mix, engine classes, age",
		Long: `Aggregate every aircraft registered to an owner (as registrant or co-owner)
from the local registry: total count, per-model counts, jet/turboprop/piston
split, state distribution, and average seats and year built. Pass --aircraft
to include the individual registrations. Requires a prior sync.`,
		Example:     "  faa-registry-pp-cli fleet report --owner \"NETJETS SALES INC\"\n  faa-registry-pp-cli fleet report --owner \"DELTA AIR LINES\" --aircraft --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagOwner == "" {
				return cmd.Help()
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			rep, err := db.Fleet(cmd.Context(), flagOwner, flagAircraft)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
		},
	}
	cmd.Flags().StringVar(&flagOwner, "owner", "", "Owner name (prefix match against registrant and co-owner names), e.g. \"NETJETS SALES INC\"")
	cmd.Flags().BoolVar(&flagAircraft, "aircraft", false, "Include the individual aircraft records in the report")
	return cmd
}
