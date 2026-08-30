// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command group: geo targeting lookups over campaign location IDs.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelGeoCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "geo",
		Short:       "Geo targeting lookups: resolve the location IDs in campaign targeting to readable place names.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelGeoResolveCmd(flags))
	return cmd
}
