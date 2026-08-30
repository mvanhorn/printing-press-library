// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDocsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "docs",
		Short:       "Search PSX regulatory documents, listing guides and notices",
		Example:     "  psx-pp-cli docs search \"rule book\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelDocsSearchCmd(flags))
	return cmd
}
