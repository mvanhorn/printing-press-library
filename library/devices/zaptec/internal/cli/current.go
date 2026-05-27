// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newCurrentCmd groups installation available-current (load-balancing) commands.
func newCurrentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Inspect and set an installation's available charging current",
	}
	cmd.AddCommand(newCurrentHeadroomCmd(flags))
	cmd.AddCommand(newCurrentSetCmd(flags))
	return cmd
}

func parseFloatField(m map[string]json.RawMessage, keys ...string) (float64, bool) {
	for _, k := range keys {
		if raw, ok := m[k]; ok {
			var f float64
			if json.Unmarshal(raw, &f) == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// newCurrentHeadroomCmd reports how many amps of the installation's breaker
// limit are uncommitted versus allocated to charging.
func newCurrentHeadroomCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "headroom <installation-id>",
		Short:       "Show uncommitted amps versus the installation's max current",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        "Fetches the installation's max current (breaker limit) and the current allocated to charging, and reports the difference — the headroom available before you hit the limit.",
		Example:     "  zaptec-pp-cli current headroom " + exampleChargerID + " --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/api/installation/{id}", "id", args[0])
			data, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("parsing installation: %w", err)
			}
			maxCurrent, hasMax := parseFloatField(m, "MaxCurrent", "MaxAllocatedCurrent")
			available, hasAvail := parseFloatField(m, "AvailableCurrent")
			var name string
			_ = json.Unmarshal(m["Name"], &name)

			result := map[string]any{
				"installation":      name,
				"max_current":       maxCurrent,
				"available_current": available,
			}
			if hasMax && hasAvail {
				result["headroom"] = round2(maxCurrent - available)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installation: %s\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "Max current:       %.1f A\n", maxCurrent)
			fmt.Fprintf(cmd.OutOrStdout(), "Available current: %.1f A\n", available)
			if hasMax && hasAvail {
				fmt.Fprintf(cmd.OutOrStdout(), "Headroom:          %.1f A\n", maxCurrent-available)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "(Headroom unavailable — installation did not report both limits.)")
			}
			return nil
		},
	}
}

// newCurrentSetCmd updates the installation's available charging current. Zaptec
// enforces a 15-minute minimum interval between such updates; the command warns
// about this before sending.
func newCurrentSetCmd(flags *rootFlags) *cobra.Command {
	var amps float64

	cmd := &cobra.Command{
		Use:   "set <installation-id> --amps <A>",
		Short: "Set the installation's available charging current (load balancing)",
		Long: `Update the installation-wide available current used for dynamic load balancing.

NOTE: Zaptec enforces a 15-minute minimum interval between available-current
updates. Sending updates more often than that will be rejected by the API.`,
		Example: "  zaptec-pp-cli current set " + exampleChargerID + " --amps 16\n  zaptec-pp-cli current set " + exampleChargerID + " --amps 10 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if amps <= 0 {
				return usageErr(fmt.Errorf("--amps must be a positive number of amperes"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if !flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: Zaptec enforces a 15-minute minimum between available-current updates.")
			}
			path := replacePathParam("/api/installation/{id}/update", "id", args[0])
			body := map[string]any{"AvailableCurrent": amps}
			data, status, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				env := map[string]any{
					"action":            "set-available-current",
					"installation":      args[0],
					"available_current": amps,
					"status":            status,
					"success":           status >= 200 && status < 300,
				}
				if flags.dryRun {
					env["dry_run"] = true
					env["status"] = 0
					env["success"] = false
				}
				if len(data) > 0 {
					var parsed any
					if json.Unmarshal(data, &parsed) == nil {
						env["data"] = parsed
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would set available current to %.1f A on installation %s\n", amps, args[0])
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set available current to %.1f A on installation %s (HTTP %d)\n", amps, args[0], status)
			return nil
		},
	}
	cmd.Flags().Float64Var(&amps, "amps", 0, "Available current in amperes")
	return cmd
}
