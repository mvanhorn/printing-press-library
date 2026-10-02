// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// Preserve absorbed provider capabilities alongside the generated novel index
// and ensure the catalog registration survives generated root refreshes.
func init() {
	whichIndex = append(whichIndex, whichEntry{Command: "bus conditions", Description: "Read route/operator baggage, boarding and cancellation conditions", Group: "Bus planning", WhyItMatters: "Check luggage and boarding policies before booking"})
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newRoutesCmd(flags))
		// Give the generated local learning tools concrete catalog wording
		// without editing their generator-owned source or request behavior.
		for path, description := range map[string]string{
			"learnings list":       "List locally taught query-to-resource mappings by query, source, resource or confidence; provider inventory is fetched separately",
			"learnings candidates": "List local CLI improvement candidates by class and status; candidates await explicit confirmation",
		} {
			command, remaining, err := root.Find(strings.Fields(path))
			if err == nil && len(remaining) == 0 && command != root {
				command.Short = description
			}
		}
	})
}
