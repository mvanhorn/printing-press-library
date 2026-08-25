// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRedirectsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "redirects",
		Short:       "redirects subcommands: audit",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRedirectsAuditCmd(flags))
	return cmd
}
