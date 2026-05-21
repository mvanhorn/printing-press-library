package cli

import (
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/jsonld"
)

// recipe get / activity get share the same JSON-LD scrape path.

func newKTRecipeGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get [slug]",
		Short:   "Get a recipe's nutrition keywords from its detail page",
		Example: `  kaloricke-tabulky-pp-cli recipe get avokadova-pomazanka-z-tvarohu --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/recepty/"+args[0], "")
			if err != nil {
				return err
			}
			return ktEmit(cmd.OutOrStdout(), flags, d)
		},
	}
	return cmd
}

func newKTActivityGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get [slug]",
		Short:   "Get an activity's detail page metadata",
		Example: `  kaloricke-tabulky-pp-cli activity get joga --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/aktivita/"+args[0], "")
			if err != nil {
				return err
			}
			return ktEmit(cmd.OutOrStdout(), flags, d)
		},
	}
	return cmd
}
