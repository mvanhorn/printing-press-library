// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelChargingCostCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB     string
		flagTariff float64
		flagSince  string
	)
	cmd := &cobra.Command{
		Use:   "charging-cost <vin>",
		Short: "Reconcile your charging sessions against your electricity tariff and see DC charge efficiency.",
		Long:  "Monetary cost and DC efficiency of charging sessions (requires --tariff). For SoC level trends use 'soc-trends'.",
		Example: "  bmw-cardata-pp-cli charging-cost WBAJB3105JUV12345 --tariff 0.32 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compute charging cost (tariff %.4f/kWh)\n", flagTariff)
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a VIN is required"))
			}
			if err := validateVIN(args[0]); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if !cmd.Flags().Changed("tariff") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tariff (price per kWh) is required"))
			}
			vin := args[0]
			since := sinceFromWindow(flagSince)
			dbPath := resolveDBPath(flagDB)
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bmw-cardata-pp-cli customers get-charging-history %s\n", dbPath, vin)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := openCardataStore(dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()
			sessions, err := listCardataChargingSessions(db, vin, since)
			if err != nil {
				return configErr(fmt.Errorf("querying charging sessions: %w", err))
			}

			var totalKWh, totalDurationH float64
			for _, s := range sessions {
				totalKWh += s.EnergyFromGridKwh
				totalDurationH += float64(s.TotalChargingDurationSec) / 3600.0
			}
			avgPower := 0.0
			if totalDurationH > 0 {
				avgPower = totalKWh / totalDurationH
			}
			view := map[string]any{
				"vin":             vin,
				"since":           flagSince,
				"session_count":   len(sessions),
				"total_kwh":       totalKWh,
				"tariff_per_kwh":  flagTariff,
				"estimated_cost":  totalKWh * flagTariff,
				"avg_charge_kw":   avgPower,
			}
			if len(sessions) == 0 {
				view["note"] = "no charging sessions in the local store for this VIN/window; run 'customers get-charging-history'"
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Charging cost for %s (last %s)\n", vin, flagSince)
			fmt.Fprintf(cmd.OutOrStdout(), "  sessions:      %d\n", len(sessions))
			fmt.Fprintf(cmd.OutOrStdout(), "  total energy:  %.2f kWh\n", totalKWh)
			fmt.Fprintf(cmd.OutOrStdout(), "  tariff:        %.4f / kWh\n", flagTariff)
			fmt.Fprintf(cmd.OutOrStdout(), "  estimated:     %.2f\n", totalKWh*flagTariff)
			fmt.Fprintf(cmd.OutOrStdout(), "  avg power:     %.2f kW\n", avgPower)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().Float64Var(&flagTariff, "tariff", 0.0, "Electricity price per kWh (required)")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Time window (e.g. 7d, 30d)")
	return cmd
}
