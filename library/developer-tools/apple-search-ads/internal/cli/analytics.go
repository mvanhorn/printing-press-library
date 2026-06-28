// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Local reporting cache: sync campaign metrics and query offline",
		Long:        `Download Apple Search Ads reporting data into a local SQLite cache, then query it offline without consuming API quota.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAnalyticsSyncCacheCmd(flags))
	cmd.AddCommand(newNovelAnalyticsQueryCmd(flags))
	return cmd
}
