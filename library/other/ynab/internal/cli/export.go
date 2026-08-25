// Copyright 2026 mikesnowbie and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "export subcommands: balances",
		Example:     "  ynab-pp-cli export balances --format projectionlab --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelExportBalancesCmd(flags))
	return cmd
}
