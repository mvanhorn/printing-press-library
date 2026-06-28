// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelOptimizeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "optimize",
		Short:       "Bid optimization suggestions and performance modeling",
		Long:        `Analyze keyword performance and compute bid adjustment suggestions to hit CPA or ROAS targets.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelOptimizeSuggestCmd(flags))
	return cmd
}
