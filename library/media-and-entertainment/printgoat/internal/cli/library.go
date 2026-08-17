// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelLibraryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "library",
		Short:       "library subcommands: doctor",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelLibraryDoctorCmd(flags))
	return cmd
}
