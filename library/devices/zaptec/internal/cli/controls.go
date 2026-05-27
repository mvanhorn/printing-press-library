// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// Friendly charger control commands. Each is a thin, named wrapper over
//   POST /api/chargers/{id}/sendCommand/{commandId}
// using the baked command IDs from zaptec_codes.go, so users send `pause`
// instead of memorizing magic numbers. All mutate charger state, support
// --dry-run (handled by the client), and emit a structured envelope under
// --json/--agent. Each command declares a literal Use: so static tooling
// (verify-skill) can discover it from source without a built binary.

const exampleChargerID = "550e8400-e29b-41d4-a716-446655440000"

func sendChargerCommand(cmd *cobra.Command, flags *rootFlags, chargerID string, commandID int, action string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	path := "/api/chargers/{id}/sendCommand/{commandId}"
	path = replacePathParam(path, "id", chargerID)
	path = replacePathParam(path, "commandId", strconv.Itoa(commandID))

	data, status, err := c.Post(path, map[string]any{})
	if err != nil {
		return classifyAPIError(err, flags)
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		env := map[string]any{
			"action":     action,
			"charger_id": chargerID,
			"command_id": commandID,
			"status":     status,
			"success":    status >= 200 && status < 300,
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
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would send %s (command %d) to charger %s\n", action, commandID, chargerID)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Sent %s to charger %s (HTTP %d)\n", action, chargerID, status)
	return nil
}

func controlExample(use string) string {
	return fmt.Sprintf("  zaptec-pp-cli %s %s\n  zaptec-pp-cli %s %s --dry-run", use, exampleChargerID, use, exampleChargerID)
}

func newPauseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "pause <charger-id>",
		Short:   "Pause charging on a charger (resumable)",
		Example: controlExample("pause"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdStopCharging, "pause")
		},
	}
}

func newResumeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "resume <charger-id>",
		Short:   "Resume a paused charging session",
		Example: controlExample("resume"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdResumeCharging, "resume")
		},
	}
}

func newStartCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "start <charger-id>",
		Short:   "Start charging on a charger",
		Example: controlExample("start"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdStartCharging, "start")
		},
	}
}

func newStopCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "stop <charger-id>",
		Short:   "Stop and finalize the charging session",
		Example: controlExample("stop"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdStopFinal, "stop")
		},
	}
}

func newRestartCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "restart <charger-id>",
		Short:   "Restart (reboot) a charger",
		Example: controlExample("restart"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdRestartCharger, "restart")
		},
	}
}

func newUnlockCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "unlock <charger-id>",
		Short:   "Unlock the charging cable connector",
		Example: controlExample("unlock"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdUnlockConnector, "unlock")
		},
	}
}

func newDeauthorizeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "deauthorize <charger-id>",
		Short:   "Deauthorize and stop the active session",
		Example: controlExample("deauthorize"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return sendChargerCommand(cmd, flags, args[0], cmdDeauthorize, "deauthorize")
		},
	}
}
