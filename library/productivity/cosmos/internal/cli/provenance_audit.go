// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelProvenanceAuditCmd(flags *rootFlags) *cobra.Command {
	var flagCollection string

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Report missing source URLs or authors and show source concentration from live collection elements.",
		Example:     "  cosmos-pp-cli provenance audit --collection 101 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "audit Cosmos provenance")
			}
			return runCosmosProvenanceAudit(cmd, flags, flagCollection)
		},
	}
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Collection ID to audit")
	return cmd
}
