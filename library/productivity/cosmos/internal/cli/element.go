// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelElementCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "element",
		Short:       "Inspect, save, connect, and explore Cosmos elements",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newCosmosElementShowCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosElementSimilarCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosElementSaveURLCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosElementConnectionsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelElementTrailCmd(flags))
	return cmd
}
