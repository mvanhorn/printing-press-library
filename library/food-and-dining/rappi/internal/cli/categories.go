// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Enumerate restaurant cuisine categories and store types",
	}
	cmd.AddCommand(newCategoriesListCmd(flags))
	cmd.AddCommand(newStoresTypesCmd(flags))
	return cmd
}

func newCategoriesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Rappi restaurant cuisine categories",
		Long: `List every Rappi cuisine category slug this CLI supports as a
closed enum. Use these slugs as the --category argument to
'restaurants list-category', 'restaurants top', and other category-
scoped commands.`,
		Example:     "  rappi-pp-cli categories list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := rappi.RestaurantCategories
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "SLUG          SPANISH         ENGLISH")
			fmt.Fprintln(out, strings.Repeat("-", 50))
			for _, c := range rows {
				fmt.Fprintf(out, "%-13s %-15s %s\n", c.Slug, c.Spanish, c.English)
			}
			return nil
		},
	}
	return cmd
}

func newStoresTypesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store-types",
		Short: "List the Rappi store-type slugs (market, farmatodo, liquor, express, rappimall-parent)",
		Long: `List every Rappi store-type slug this CLI supports as a closed
enum. Use these as the --type argument to 'stores list-by-type',
'stores coverage', 'stores adjacency', and other type-scoped commands.`,
		Example:     "  rappi-pp-cli categories store-types --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := rappi.StoreTypes
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "SLUG               SPANISH          ENGLISH")
			fmt.Fprintln(out, strings.Repeat("-", 55))
			for _, t := range rows {
				fmt.Fprintf(out, "%-18s %-16s %s\n", t.Slug, t.Spanish, t.English)
			}
			return nil
		},
	}
	return cmd
}
