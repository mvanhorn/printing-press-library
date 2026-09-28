// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSerpCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "serp",
		Short:       "Track Google results pages for a query over time.",
		Example:     "  serply-pp-cli serp diff --q 'best static site generator' --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSerpDiffCmd(flags))
	return cmd
}
