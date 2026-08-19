// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Group command for the formulas family; the leaves carry the behavior.

package cli

import "github.com/spf13/cobra"

func newNovelFormulasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formulas",
		Short: "Generate Clay formulas from natural language",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:parent-group":     "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelFormulasGenerateCmd(flags))
	return cmd
}
