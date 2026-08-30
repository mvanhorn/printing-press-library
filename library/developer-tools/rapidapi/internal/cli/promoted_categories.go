// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic categories command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCategoriesPromotedCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:     "categories [flags]",
		Short:   "List top-level marketplace categories with weights and descriptions",
		Long:    "List top-level marketplace categories from the RapidAPI hub, ordered by weight (Cybersecurity, Cryptography, Movies, Jobs, Energy, ...).",
		Example: "  rapidapi-pp-cli categories --limit 20",
		Annotations: map[string]string{"pp:endpoint": "categories.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			variables := map[string]any{"limit": limit}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getCategoriesByCtx", variables, gqlResponsePaths["getCategoriesByCtx"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true, "weight": true, "thumbnail": true, "shortDescription": true, "slugifiedName": true, "color": true})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of categories")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
