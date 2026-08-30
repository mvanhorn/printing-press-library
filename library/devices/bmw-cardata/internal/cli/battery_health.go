// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source local

const cardataMaxEnergyDescriptor = "vehicle.drivetrain.batteryManagement.maxEnergy"

func newNovelBatteryHealthCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:   "battery-health <vin>",
		Short: "Track battery capacity degradation by comparing observed max energy against the nameplate capacity.",
		Long:  "Capacity degradation (observed vs nameplate). For SoC level over time use 'soc-trends'.",
		Example: "  bmw-cardata-pp-cli battery-health WBAJB3105JUV12345 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute battery health")
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
			vin := args[0]
			dbPath := resolveDBPath(flagDB)
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bmw-cardata-pp-cli customers get-basic-data %s && customers get-telematic-data %s --container-id <id>\n", dbPath, vin, vin)
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

			vehicles, _ := listCardataVehicles(db)
			var nameplate float64
			for _, v := range vehicles {
				if v.VIN == vin {
					nameplate, _ = strconv.ParseFloat(v.HvsMaxEnergyAbs, 64)
					break
				}
			}
			// Fallback nameplate from the battery-size descriptor snapshot.
			if nameplate <= 0 {
				if p, ok, _ := snapshotValue(db, vin, "vehicle.drivetrain.batteryManagement.batterySizeMax"); ok {
					nameplate, _ = strconv.ParseFloat(p.Value, 64)
				}
			}

			// Max observed energy across the time-series.
			var maxObserved float64
			var observedTS string
			rows, err := db.DB().Query(
				`SELECT value, COALESCE(ts,'') FROM cardata_telematic_snapshots
				 WHERE vin = ? AND descriptor = ? ORDER BY id DESC LIMIT 200`, vin, cardataMaxEnergyDescriptor)
			if err != nil {
				return configErr(fmt.Errorf("querying max-energy history: %w", err))
			}
			for rows.Next() {
				var v, ts string
				if err := rows.Scan(&v, &ts); err != nil {
					rows.Close()
					return configErr(err)
				}
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > maxObserved {
					maxObserved, observedTS = f, ts
				}
			}
			rows.Close()

			view := map[string]any{
				"vin":                   vin,
				"nameplate_kwh":         nameplate,
				"max_observed_kwh":      maxObserved,
				"max_observed_at":       observedTS,
			}
			switch {
			case nameplate <= 0 && maxObserved <= 0:
				view["note"] = "no nameplate (basicData) or observed energy data; fetch both first"
			case nameplate <= 0:
				view["note"] = "no nameplate capacity (run customers get-basic-data); cannot compute degradation"
			default:
				degradation := (nameplate - maxObserved) / nameplate * 100.0
				if degradation < 0 {
					degradation = 0
				}
				view["degradation_pct"] = degradation
				view["health_pct"] = 100.0 - degradation
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Battery health for %s\n", vin)
			fmt.Fprintf(cmd.OutOrStdout(), "  nameplate:     %.2f kWh\n", nameplate)
			fmt.Fprintf(cmd.OutOrStdout(), "  max observed:  %.2f kWh\n", maxObserved)
			if dp, ok := view["degradation_pct"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  degradation:   %.2f%%\n", dp)
			}
			if n, ok := view["note"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  note: %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}
