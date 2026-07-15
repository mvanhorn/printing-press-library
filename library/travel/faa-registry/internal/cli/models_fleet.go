// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelModelsFleetCmd(flags *rootFlags) *cobra.Command {
	var flagManufacturer string
	var flagModel string

	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Break down every registered example of a make/model by registrant type and state",
		Long: `Aggregate the local registry for a make (and optional model): total
registered examples, split by registrant type (corporation, individual, LLC,
co-owned, government) and by state, with the year-built range. A market-
research query the one-at-a-time FAA inquiry pages cannot express. Requires
a prior sync.`,
		Example:     "  faa-registry-pp-cli models fleet --manufacturer CIRRUS --model SR22\n  faa-registry-pp-cli models fleet --manufacturer GULFSTREAM --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagManufacturer == "" {
				return cmd.Help()
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			rep, err := db.ModelFleet(cmd.Context(), flagManufacturer, flagModel)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
		},
	}
	cmd.Flags().StringVar(&flagManufacturer, "manufacturer", "", "Aircraft manufacturer (prefix match), e.g. CIRRUS")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model (prefix match), e.g. SR22")
	return cmd
}
