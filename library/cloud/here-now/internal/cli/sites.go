// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command group: `sites`. Parents the local-mirror site
// maintenance subcommands (currently `sites stale`). Header is a plain
// copyright line so regen-merge preserves this file.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSitesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sites",
		Short:       "Local-mirror site maintenance (e.g. find stale sites)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSitesStaleCmd(flags))
	return cmd
}
