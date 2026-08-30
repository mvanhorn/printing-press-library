// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelCollectionOverlapCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "overlap <left-collection-id> <right-collection-id>",
		Short:       "Compare two collections and report shared elements, duplicate media, and references unique to each side.",
		Example:     "  cosmos-pp-cli collection overlap 101 202 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare Cosmos collection overlap")
			}
			return runCosmosCollectionOverlap(cmd, flags, args)
		},
	}
	return cmd
}
