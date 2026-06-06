// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Track watched products and see price changes since the last index refresh",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWatchAddCmd(flags))
	cmd.AddCommand(newNovelWatchStatusCmd(flags))
	return cmd
}
