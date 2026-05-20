// Copyright 2026 conduyt. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "insights",
		Short:       "Compound analytics across contacts, deals, pipelines, and email",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	cmd.AddCommand(newInsightsContactTrendsCmd(flags))
	cmd.AddCommand(newInsightsDealVelocityCmd(flags))
	cmd.AddCommand(newInsightsPipelineHealthCmd(flags))
	cmd.AddCommand(newInsightsEmailStatsCmd(flags))

	return cmd
}
