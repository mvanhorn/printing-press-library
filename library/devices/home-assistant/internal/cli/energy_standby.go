// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Standby-load discovery workflow.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func newNovelEnergyStandbyCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "standby",
		Short:       "Find devices drawing persistent power during explicit periods when related household states show inactivity.",
		Example:     "--since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			var loads []map[string]any
			for _, s := range states {
				id := entityID(s)
				if !strings.Contains(id, "power") && !strings.Contains(id, "energy") {
					continue
				}
				value, err := strconv.ParseFloat(stateValue(s), 64)
				if err != nil || value <= 0 {
					continue
				}
				loads = append(loads, map[string]any{"entity_id": id, "friendly_name": friendlyName(s), "value": value, "unit": attributes(s)["unit_of_measurement"]})
			}
			return workflowOutput(map[string]any{"candidate_loads": loads, "count": len(loads), "note": "Live positive power/energy candidates; sync history to correlate explicit inactivity windows."}, flags, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Requested reporting window; sync history for time-window correlation")
	return cmd
}
