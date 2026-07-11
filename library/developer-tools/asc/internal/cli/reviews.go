// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelReviewsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "reviews",
		Short:       "reviews subcommands: recent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelReviewsRecentCmd(flags))
	return cmd
}
