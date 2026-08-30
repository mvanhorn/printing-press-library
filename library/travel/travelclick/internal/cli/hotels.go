// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHotelsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hotels",
		Short:       "Local state that compounds",
		Example:     "  travelclick-pp-cli hotels alias add made-nyc 102306",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHotelsAliasCmd(flags))
	return cmd
}
