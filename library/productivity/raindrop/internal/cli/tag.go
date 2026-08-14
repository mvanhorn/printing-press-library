// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTagCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "tag",
		Short:       "Analyze and safely clean local tag vocabulary",
		Example:     "  raindrop-pp-cli tag health --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTagHealthCmd(flags))
	addNovelCommandIfAbsent(cmd, newTagPlanMergesCmd(flags))
	addNovelCommandIfAbsent(cmd, newTagApplyPlanCmd(flags))
	return cmd
}
