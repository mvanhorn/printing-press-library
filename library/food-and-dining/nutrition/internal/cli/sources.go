// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/source"

	"github.com/spf13/cobra"
)

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List the data sources this CLI aggregates",
		Long: "This CLI unifies two peer data sources behind one food model: USDA FoodData " +
			"Central (official API) and NutritionValue.org (derived analytics via HTML). Use " +
			"'sources list' to see them and their auth requirements.",
		Example:     "  nutrition-pp-cli sources list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSourcesListCmd(flags))
	return cmd
}

func newSourcesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List registered data sources",
		Example:     "  nutrition-pp-cli sources list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would list registered sources")
			}
			sources := source.All()
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{
				"count":   len(sources),
				"sources": sources,
			}, flags)
		},
	}
	return cmd
}
