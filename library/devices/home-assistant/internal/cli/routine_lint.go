// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Live routine health workflow.
// pp:data-source live

package cli

import "github.com/spf13/cobra"

func newNovelRoutineLintCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "lint",
		Short:       "Find missing entities, unavailable services, disabled dependencies, ambiguous names, and stale routine references.",
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
			var findings []map[string]any
			for _, routine := range states {
				id := entityID(routine)
				if id[:min(len(id), 11)] != "automation." && id[:min(len(id), 7)] != "script." && id[:min(len(id), 6)] != "scene." {
					continue
				}
				if isUnavailable(routine) {
					findings = append(findings, map[string]any{"routine": id, "severity": "warning", "kind": "unavailable", "message": "routine entity is unavailable"})
				}
			}
			return workflowOutput(map[string]any{"findings": findings, "checked_states": len(states), "note": "Live-state lint; sync configuration snapshots for reference-level analysis."}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
