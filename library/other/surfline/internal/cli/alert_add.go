// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: store a surf-condition alert rule locally.
//
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAlertAddCmd(flags *rootFlags) *cobra.Command {
	var spot string
	var minSurf, minPeriod, maxWind, minRating float64
	var offshore bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Store a swell/wind/tide threshold rule for later `alert run` evaluation.",
		Example: strings.Trim(`
  surfline-pp-cli alert add dawn --spot 5842041f4e65fad6a7708807 --min-period 12 --max-wind 10 --offshore
  surfline-pp-cli alert add fun --spot 5842041f4e65fad6a7708807 --min-surf 3 --min-rating 4`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would store the alert rule")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an alert name is required"))
			}
			if spot == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--spot is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rule := alertRule{
				Name:         args[0],
				SpotID:       spot,
				MinSurf:      minSurf,
				MinPeriod:    minPeriod,
				MaxWind:      maxWind,
				OffshoreOnly: offshore,
				MinRating:    minRating,
			}
			if err := saveAlertRule(ctx, db, rule, time.Now().Unix()); err != nil {
				return fmt.Errorf("saving alert: %w", err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rule, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved alert %q for spot %s\n", rule.Name, rule.SpotID)
			return nil
		},
	}
	cmd.Flags().StringVar(&spot, "spot", "", "Spot ID to watch (required)")
	cmd.Flags().Float64Var(&minSurf, "min-surf", 0, "Minimum surf height (max, ft) to trigger")
	cmd.Flags().Float64Var(&minPeriod, "min-period", 0, "Minimum primary swell period (s) to trigger")
	cmd.Flags().Float64Var(&maxWind, "max-wind", 0, "Maximum wind speed (kts); above this the rule will not trigger")
	cmd.Flags().BoolVar(&offshore, "offshore", false, "Require offshore wind to trigger")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Minimum numeric rating (1=VERY_POOR .. 7=EPIC) to trigger")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	return cmd
}
