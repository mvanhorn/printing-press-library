// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDocCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Fetch and project Apple doc pages",
		Long:  "Fetch and project Apple doc pages. The 'doc get' subcommand supports --shape projection and --markdown rendering.",
		Example: "  apple-docs-pp-cli doc get swiftui/view --shape signature --agent\n" +
			"  apple-docs-pp-cli doc get 'swiftui/view/onappear(perform:)' --markdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelDocGetCmd(flags))
	return cmd
}
