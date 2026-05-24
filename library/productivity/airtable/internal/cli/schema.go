// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newSchemaCmd is the parent for hand-built schema subcommands
// (dump, diff, drift). Generated commands live under `bases` (e.g.
// `bases get_schema`); this parent groups the local-mirror-aware and
// multi-base novel tooling that the absorb manifest specified.
func newSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema tooling — dump, diff, and multi-base drift detection",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSchemaDumpCmd(flags))
	cmd.AddCommand(newSchemaDiffCmd(flags))
	cmd.AddCommand(newSchemaDriftCmd(flags))
	return cmd
}
