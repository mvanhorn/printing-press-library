// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEcliCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecli",
		Short: "Parse, validate, and resolve ECLI identifiers (NL / EU / CE-ECHR)",
		Long: `ECLI utilities: parse a raw ECLI string into its parts, emit the canonical
deeplink, and validate well-formedness — all entirely offline.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelEcliParseCmd(flags))
	cmd.AddCommand(newNovelEcliURLCmd(flags))
	return cmd
}
