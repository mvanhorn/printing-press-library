// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRecipesLinkCmd(flags *rootFlags) *cobra.Command {
	var recipeName string
	cmd := &cobra.Command{
		Use:         "link",
		Short:       "Show a recipe's source URL",
		Example:     "  anylist-pp-cli recipes link --name Pancakes --dry-run",
		Annotations: map[string]string{"pp:endpoint": "recipes.link", "pp:method": "GET", "pp:path": "local-cache", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			recipeName = strings.TrimSpace(recipeName)
			if recipeName == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "recipe": recipeName}, "Dry run: would read the recipe source URL from the local cache\n")
			}
			_, st, err := openLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			recipe, err := st.FindRecipeByName(recipeName)
			if err != nil {
				return err
			}
			if strings.TrimSpace(recipe.SourceURL) == "" {
				return fmt.Errorf("recipe %q has no source URL", recipe.Name)
			}
			result := map[string]any{"recipe": recipe.Name, "source_url": recipe.SourceURL}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), recipe.SourceURL)
			return nil
		},
	}
	cmd.Flags().StringVarP(&recipeName, "name", "n", "", "Recipe name")
	return cmd
}
