// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRenderCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "render",
		Short:       "Read the local render log: past renders, spend by voice or model, and cost diffs",
		Example:     "  fish-audio-pp-cli render diff 1 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelRenderDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelRenderLogCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelRenderSpendCmd(flags))
	return cmd
}
