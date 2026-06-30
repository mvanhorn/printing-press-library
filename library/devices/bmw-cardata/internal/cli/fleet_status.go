// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source local

const (
	cardataLatDescriptor     = "vehicle.cabin.infotainment.navigation.currentLocation.latitude"
	cardataLongDescriptor    = "vehicle.cabin.infotainment.navigation.currentLocation.longitude"
	cardataChargingStatus    = "vehicle.drivetrain.electricEngine.charging.status"
)

func newNovelFleetStatusCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "One table of current SoC, range, charging state and last location across every vehicle on your account.",
		Long:  "Multi-VIN current snapshot across all mapped vehicles. For a single VIN's history over time use 'soc-trends'.",
		Example: "  bmw-cardata-pp-cli fleet status --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query fleet status across local vehicles")
				return nil
			}
			dbPath := resolveDBPath(flagDB)
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bmw-cardata-pp-cli customers get-mappings\n", dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := openCardataStore(dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()
			vehicles, err := listCardataVehicles(db)
			if err != nil {
				return configErr(fmt.Errorf("listing vehicles: %w", err))
			}
			rows := make([]map[string]any, 0, len(vehicles))
			for _, v := range vehicles {
				snap, _ := latestCardataSnapshot(db, v.VIN)
				lookup := map[string]telematicPoint{}
				for _, p := range snap {
					lookup[p.Descriptor] = p
				}
				row := map[string]any{
					"vin":         v.VIN,
					"brand":       v.Brand,
					"model":       v.ModelName,
				}
				socDesc := pickDescriptor(lookup, cardataSocDescriptors)
				if p, ok := lookup[socDesc]; ok {
					row["soc"], _ = strconv.ParseFloat(p.Value, 64)
					row["soc_unit"] = p.Unit
				}
				if p, ok := lookup[cardataRangeDescriptor]; ok {
					row["range_km"], _ = strconv.ParseFloat(p.Value, 64)
				}
				if p, ok := lookup[cardataChargingStatus]; ok {
					row["charging_status"] = p.Value
				}
				if lat, ok1 := lookup[cardataLatDescriptor]; ok1 {
					if lng, ok2 := lookup[cardataLongDescriptor]; ok2 {
						row["location"] = map[string]string{
							"lat": lat.Value, "lng": lng.Value, "ts": lat.Timestamp,
						}
					}
				}
				rows = append(rows, row)
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no vehicles in local store; run 'customers get-mappings'")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-17s %-10s %-8s %-8s %-12s\n", "VIN", "Model", "SoC%", "Range", "Charging")
			for _, r := range rows {
				soc := "-"
				if s, ok := r["soc"]; ok {
					soc = fmt.Sprintf("%.0f", s)
				}
				rng := "-"
				if s, ok := r["range_km"]; ok {
					rng = fmt.Sprintf("%.0f", s)
				}
				cs := "-"
				if s, ok := r["charging_status"]; ok {
					cs = fmt.Sprintf("%v", s)
				}
				model := "-"
				if s, ok := r["model"]; ok {
					model = fmt.Sprintf("%v", s)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-17s %-10s %-8s %-8s %-12s\n", r["vin"], model, soc, rng, cs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// pickDescriptor returns the first descriptor from preferred that exists in m.
func pickDescriptor(m map[string]telematicPoint, preferred []string) string {
	for _, d := range preferred {
		if _, ok := m[d]; ok {
			return d
		}
	}
	if len(preferred) > 0 {
		return preferred[0]
	}
	return ""
}
