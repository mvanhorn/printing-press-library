// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelResolveCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "resolve",
		Short:       "resolve subcommands: facility",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelResolveFacilityCmd(flags))
	return cmd
}
