// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: sede command group — preserve on regeneration.

package cli

import "github.com/spf13/cobra"

func newSedeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sede",
		Short: "Ricerca enti, AOO e UO per indirizzo sede (portale IPA)",
	}
	cmd.AddCommand(newSedeEntiCmd(flags))
	cmd.AddCommand(newSedeAooCmd(flags))
	cmd.AddCommand(newSedeUoCmd(flags))
	return cmd
}
