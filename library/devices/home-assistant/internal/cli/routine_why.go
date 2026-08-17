// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Routine incident evidence workflow.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

func newNovelRoutineWhyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "why <automation-or-script>",
		Short:       "Reconstruct why an automation or script failed from traces, state changes, logbook entries, and errors.",
		Example:     "automation.movie_night --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("provide an automation or script entity_id"))
			}
			if dryRunOK(flags) {
				return nil
			}
			states, err := householdStates(cmd.Context(), flags)
			if err != nil {
				return err
			}
			routine, err := matchEntity(states, args[0])
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			logs, err := c.Get(cmd.Context(), "/api/logbook/"+time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339), map[string]string{"entity": entityID(routine)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return workflowOutput(map[string]any{"routine": routine, "window": "24h", "logbook": json.RawMessage(logs), "explanation": "Evidence is returned verbatim; inspect the latest trace and logbook rows for the failed branch."}, flags, cmd.OutOrStdout())
		},
	}
	return cmd
}
