// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// pp:data-source local

// validateVIN returns nil if s looks like a BMW VIN (17 chars, alphanum, no I/O/Q),
// otherwise a usage-friendly error. Used by the transcendence commands to
// reject invalid input at the door rather than returning an empty "no data"
// result that looks healthy.
func validateVIN(s string) error {
	if len(s) != 17 {
		return fmt.Errorf("VIN must be 17 characters, got %d", len(s))
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("VIN must be uppercase alphanumeric, got %q", s)
		}
		if r == 'I' || r == 'O' || r == 'Q' {
			return fmt.Errorf("VIN cannot contain %q", r)
		}
	}
	return nil
}

var cardataSocDescriptors = []string{
	"vehicle.powertrain.electric.battery.stateOfCharge.displayed",
	"vehicle.drivetrain.batteryManagement.header",
}
const cardataRangeDescriptor = "vehicle.drivetrain.electricEngine.remainingElectricRange"

func newNovelSocTrendsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB     string
		flagWindow string
	)
	cmd := &cobra.Command{
		Use:   "soc-trends <vin>",
		Short: "See your battery state-of-charge and derived range as a time series you can chart.",
		Long: "Use for SoC and derived range as a numeric series over time. For capacity degradation vs nameplate use 'battery-health'; for monetary charging cost use 'charging-cost'.",
		Example: "  bmw-cardata-pp-cli soc-trends WBAJB3105JUV12345 --window 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local SoC/range trend")
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
			since := sinceFromWindow(flagWindow)

			dbPath := resolveDBPath(flagDB)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bmw-cardata-pp-cli customers get-telematic-data %s --container-id <id>\n", dbPath, vin)
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

			// Pick whichever SoC descriptor has data.
			var socDesc string
			var socSeries []telematicPoint
			for _, d := range cardataSocDescriptors {
				pts, err := cardataSnapshotSeries(db, vin, d, since)
				if err != nil {
					return configErr(fmt.Errorf("querying %s: %w", d, err))
				}
				if len(pts) > 0 {
					socDesc, socSeries = d, pts
					break
				}
			}
			rangeSeries, err := cardataSnapshotSeries(db, vin, cardataRangeDescriptor, since)
			if err != nil {
				return configErr(fmt.Errorf("querying range: %w", err))
			}

			view := map[string]any{
				"vin":             vin,
				"window":          humanWindow(flagWindow),
				"soc_descriptor":  socDesc,
				"soc_series":      socSeries,
				"range_series":    rangeSeries,
				"soc_samples":     len(socSeries),
				"range_samples":   len(rangeSeries),
			}
			if len(socSeries) == 0 && len(rangeSeries) == 0 {
				view["note"] = "no telematic snapshots in the local store for this VIN/window; fetch live data first"
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "SoC/range trend for %s (last %s)\n", vin, humanWindow(flagWindow))
			if socDesc == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "  no SoC data")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  SoC (%s): %d samples\n", socDesc, len(socSeries))
				for _, p := range socSeries {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s %s\n", p.Timestamp, p.Value, p.Unit)
				}
			}
			if len(rangeSeries) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Range: %d samples\n", len(rangeSeries))
				for _, p := range rangeSeries {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s %s\n", p.Timestamp, p.Value, p.Unit)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Time window (e.g. 7d, 30d)")
	return cmd
}
