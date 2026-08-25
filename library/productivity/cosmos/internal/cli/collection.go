// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCollectionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "collection",
		Short:       "List, inspect, create, connect, compare, and audit Cosmos collections",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newCosmosCollectionListCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionShowCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionElementsCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionSearchCmd(flags))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionCreateCmd(flags, false))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionCreateCmd(flags, true))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionConnectionCmd(flags, false))
	addNovelCommandIfAbsent(cmd, newCosmosCollectionConnectionCmd(flags, true))
	addNovelCommandIfAbsent(cmd, newNovelCollectionCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCollectionOverlapCmd(flags))
	return cmd
}
