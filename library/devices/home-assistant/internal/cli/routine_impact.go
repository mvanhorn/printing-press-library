// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Routine dependency inspection workflow.
// pp:data-source live

package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelRoutineImpactCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "impact <entity-or-routine>",
		Short:       "See every automation, script, scene, helper, and service target affected by changing one entity or routine.",
		Example:     "light.living_room --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("provide an entity or routine id"))
			}
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			refs := routineReferences(states, args[0])
			return workflowOutput(map[string]any{"target": args[0], "dependents": refs, "count": len(refs), "source": "live_state_attributes"}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
