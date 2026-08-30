// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelVoiceCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "voice",
		Short:       "Clone, design, discover, and verify Fish Audio voice models",
		Example:     "  fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelVoiceVerifyCmd(flags))
	return cmd
}
