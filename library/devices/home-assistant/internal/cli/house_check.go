// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Household safety and comfort exception workflow.
// pp:data-source live

package cli

import "github.com/spf13/cobra"

func newNovelHouseCheckCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "check",
		Short:       "See open windows, unlocked locks, lights left on, low batteries, and unavailable safety devices in one answer.",
		Example:     "--agent --select category,entity_id,friendly_name,area,state,severity",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			var exceptions []map[string]any
			for _, s := range states {
				id, value := entityID(s), stateValue(s)
				category, severity := "", ""
				switch {
				case (len(id) > 14 && id[:14] == "binary_sensor.") && (value == "on"):
					category, severity = "open_or_active_sensor", "warning"
				case len(id) > 5 && id[:5] == "lock." && value == "unlocked":
					category, severity = "unlocked_lock", "warning"
				case len(id) > 6 && id[:6] == "light." && value == "on":
					category, severity = "light_on", "info"
				case isUnavailable(s):
					category, severity = "unavailable", "warning"
				}
				if category != "" {
					exceptions = append(exceptions, map[string]any{"category": category, "entity_id": id, "friendly_name": friendlyName(s), "state": value, "severity": severity})
				}
			}
			return workflowOutput(map[string]any{"exceptions": exceptions, "count": len(exceptions)}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
