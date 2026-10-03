// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/store"
	"github.com/spf13/cobra"
	"strconv"
)

func newSpotsInspectCmd(flags *rootFlags) *cobra.Command {
	var id int64
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "inspect [spot-id]", Short: "Inspect recorded accessibility questions, counts, source dates and gaps.",
		Long:        "Inspect one public source spot ID. Positive reports express a crowd observation, and mixed or missing reports remain explicit. For an explicit candidate set use spots compare; for discovery use spots search.",
		Example:     "  wheelog-pp-cli spots inspect 166345 --require-question 102 --agent\n  wheelog-pp-cli spots inspect --id 166345 --data-source local --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "spot-id=166345"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inspect public WheeLog spot")
			}
			if len(args) > 1 || (len(args) > 0 && cmd.Flags().Changed("id")) {
				return usageErr(fmt.Errorf("provide exactly one spot ID as a positional value or --id"))
			}
			if len(args) == 1 {
				parsed, err := wheelogID(args[0])
				if err != nil {
					return err
				}
				id = parsed
			}
			if id == 0 {
				if !flags.asJSON && len(args) == 0 && !hasChangedLocalFlags(cmd) {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("a spot ID is required; use inspect 166345 or --id 166345"))
			}
			if _, err := wheelogID(strconv.FormatInt(id, 10)); err != nil {
				return err
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var cached []store.WheelogObservation
			var c *client.Client
			if mode == "local" {
				cached, err = savedWheelog(ctx, options.DB)
				if err != nil {
					return err
				}
				if mode == "local" {
					wheelogLocalHint(cmd, cached, maxAge)
				}
			}
			if mode != "local" {
				c, err = newWheelogClient(flags)
				if err != nil {
					return err
				}
			}
			spot, err := resolveWheelogSpot(ctx, flags, options, id, c, cached)
			if err != nil {
				return wheelogError(err)
			}
			return emitWheelog(cmd, flags, viewWheelog(spot, options.Questions, maxAge), spot.Source)
		}}
	cmd.Flags().Int64Var(&id, "id", 0, "Stable public WheeLog spot ID; use instead of the positional ID.")
	addWheelogReadFlags(cmd, &options)
	return cmd
}
