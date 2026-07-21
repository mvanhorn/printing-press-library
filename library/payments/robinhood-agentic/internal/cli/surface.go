// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSurfaceCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "surface",
		Short:       "Track Robinhood's beta MCP tool surface (capture, diff)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSurfaceCaptureCmd(flags))
	cmd.AddCommand(newNovelSurfaceDiffCmd(flags))
	return cmd
}
