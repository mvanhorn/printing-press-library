// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelVehiclesDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB    string
		flagSince string
	)
	cmd := &cobra.Command{
		Use:   "diff <vin>",
		Short: "Show exactly what changed between two telematic snapshots (windows, charging, locks, location).",
		Long:  "Discrete state changes between two snapshots. For a numeric level series over time use 'soc-trends'.",
		Example: "  bmw-cardata-pp-cli vehicles diff WBAJB3105JUV12345 --since 24h",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff telematic snapshots")
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
			cutoff := sinceFromWindow(flagSince)
			dbPath := resolveDBPath(flagDB)
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
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

			latest, err := latestCardataSnapshot(db, vin)
			if err != nil {
				return configErr(fmt.Errorf("querying latest snapshot: %w", err))
			}
			prev, err := snapshotAsOf(db, vin, cutoff)
			if err != nil {
				return configErr(fmt.Errorf("querying prior snapshot: %w", err))
			}
			prevMap := map[string]telematicPoint{}
			for _, p := range prev {
				prevMap[p.Descriptor] = p
			}

			changes := make([]map[string]any, 0)
			for _, p := range latest {
				old, hadPrev := prevMap[p.Descriptor]
				if !hadPrev {
					continue // newly seen since cutoff — not a state change
				}
				if old.Value != p.Value {
					changes = append(changes, map[string]any{
						"descriptor": p.Descriptor,
						"from":        old.Value,
						"to":          p.Value,
						"unit":        p.Unit,
						"changed_at":  p.Timestamp,
					})
				}
			}
			view := map[string]any{
				"vin":         vin,
				"since":       cutoff.Format(time.RFC3339),
				"changes":     changes,
				"change_count": len(changes),
			}
			if len(changes) == 0 {
				view["note"] = fmt.Sprintf("no state changes since %s", cutoff.Format("2006-01-02 15:04 UTC"))
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Changes for %s since %s\n", vin, cutoff.Format("2006-01-02 15:04 UTC"))
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  no state changes")
				return nil
			}
			for _, c := range changes {
				unit := ""
				if u, ok := c["unit"].(string); ok && u != "" {
					unit = " " + u
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v -> %v%s\n", c["descriptor"], c["from"], c["to"], unit)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Compare latest snapshot against the one at this age (e.g. 24h, 7d)")
	return cmd
}
