// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelKeywordsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "keywords",
		Short:       "Keyword management: auto-promote converting search terms to targeted keywords",
		Long:        `Manage keywords across campaigns and ad groups. The auto-promote subcommand analyzes search term reports and promotes high-converting terms to targeting keywords with smart match-type routing.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelKeywordsAutoPromoteCmd(flags))
	return cmd
}
