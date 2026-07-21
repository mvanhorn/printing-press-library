// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Previewable household mode workflow.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelModeRunCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "run <scene-or-script>",
		Short:       "Preview a scene or script, run it, and verify every resulting entity state.",
		Example:     "movie-night --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("provide exactly one scene or script entity_id"))
			}
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			target, err := matchEntity(states, args[0])
			if err != nil {
				return err
			}
			id := entityID(target)
			if !strings.HasPrefix(id, "scene.") && !strings.HasPrefix(id, "script.") {
				return usageErr(fmt.Errorf("%s is not a scene or script", id))
			}
			plan := map[string]any{"target": id, "current_state": stateValue(target), "service": strings.SplitN(id, ".", 2)[0] + ".turn_on"}
			if flags.dryRun {
				return workflowOutput(map[string]any{"dry_run": true, "plan": plan}, flags, cmd.OutOrStdout())
			}
			domain := strings.SplitN(id, ".", 2)[0]
			result, err := callHAService(cmd.Context(), flags, domain, "turn_on", map[string]any{"entity_id": id})
			if err != nil {
				return err
			}
			var changed []map[string]any
			if err := json.Unmarshal(result, &changed); err != nil {
				return fmt.Errorf("decode service response for verification: %w", err)
			}
			if len(changed) == 0 {
				return fmt.Errorf("mode run service returned no changed states to verify")
			}
			current, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			verified := make([]map[string]any, 0, len(changed))
			for _, state := range changed {
				entity, _ := state["entity_id"].(string)
				if entity == "" {
					return fmt.Errorf("mode run service returned a changed state without entity_id")
				}
				actual, matchErr := matchEntity(current, entity)
				if matchErr != nil {
					return fmt.Errorf("mode run could not verify %s: %w", entity, matchErr)
				}
				if expected := stateValue(state); expected != "" && stateValue(actual) != expected {
					return fmt.Errorf("mode run verification mismatch for %s: got %q, want %q", entity, stateValue(actual), expected)
				}
				verified = append(verified, actual)
			}
			verifiedOK := len(changed) > 0 && len(verified) == len(changed)
			if !verifiedOK {
				return fmt.Errorf("mode run could not verify every resulting entity state")
			}
			return workflowOutput(map[string]any{"plan": plan, "service_result": json.RawMessage(result), "verified_states": verified, "verified": verifiedOK}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
