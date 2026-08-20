// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPlaylistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "playlist",
		Short:  "Your playlists",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPlaylistListCmd(flags))
	return cmd
}
