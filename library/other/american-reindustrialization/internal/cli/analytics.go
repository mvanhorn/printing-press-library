// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "analytics",
		Short:       "Cross-entity analytics over the locally synced directory",
		Long:        "Run sector, geographic, funding, and other crosstab analytics against the locally synced company directory. Requires `sync` to have populated the local store.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	return cmd
}
