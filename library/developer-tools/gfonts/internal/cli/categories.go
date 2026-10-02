package cli

import "github.com/spf13/cobra"

func newCategoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "categories",
		Aliases: []string{"cats"},
		Short:   "List all font categories",
		Example: "  gfonts-pp-cli categories",
		Args:    cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			cmdCategories(args)
		},
	}
}
