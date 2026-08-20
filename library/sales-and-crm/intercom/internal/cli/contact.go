// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelContactCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		// PATCH(intercom-contact-parent-use): variadic Use so verify-skill's
		// example parser accepts `contact 360 <key>` (its parser does not descend
		// into the numeric `360` subcommand). Carried from the prior published CLI.
		Use:         "contact <subcommand> [args...]",
		Hidden:      true,
		Short:       "Cross-entity views centered on one contact (use 'contacts' for resource CRUD)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelContact360Cmd(flags))
	return cmd
}
