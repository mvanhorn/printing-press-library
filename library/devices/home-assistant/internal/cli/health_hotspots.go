// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Maintenance hotspot workflow.
// pp:data-source live

package cli

import "github.com/spf13/cobra"

func newNovelHealthHotspotsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hotspots",
		Short:       "Group recurring unavailable, stale, low-battery, and disconnecting entities into device, room, integration",
		Example:     "--agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			hotspots := map[string][]string{}
			for _, s := range states {
				if !isUnavailable(s) {
					continue
				}
				key := entityID(s)
				if device, ok := attributes(s)["device_id"].(string); ok && device != "" {
					key = "device:" + device
				}
				hotspots[key] = append(hotspots[key], entityID(s))
			}
			return workflowOutput(map[string]any{"hotspots": hotspots, "kind": "unavailable_or_unknown", "source": "live_states"}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
