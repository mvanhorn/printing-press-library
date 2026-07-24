// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRewardCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "reward",
		Short:       "reward subcommands: afford",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRewardAffordCmd(flags))
	return cmd
}
