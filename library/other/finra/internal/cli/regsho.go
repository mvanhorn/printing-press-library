// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRegshoCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "regsho",
		Short:       "regsho subcommands: threshold-watch, volume",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRegshoThresholdWatchCmd(flags))
	cmd.AddCommand(newNovelRegshoVolumeCmd(flags))
	return cmd
}
