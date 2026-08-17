// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Current household recap workflow.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelHouseRecapCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "recap",
		Short:       "Review factual household activity, routine failures, presence changes, openings",
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
			byDomain, unavailable := map[string]int{}, 0
			for _, s := range states {
				id := entityID(s)
				domain := strings.SplitN(id, ".", 2)[0]
				byDomain[domain]++
				if isUnavailable(s) {
					unavailable++
				}
			}
			return workflowOutput(map[string]any{"captured_at": time.Now().UTC(), "entities_by_domain": byDomain, "unavailable_entities": unavailable, "entity_count": len(states), "note": "Factual current-state recap; run sync before requesting historical summaries."}, flags, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Requested reporting window; current live-state recap reports its capture time")
	return cmd
}
