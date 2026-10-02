package cli

import "github.com/spf13/cobra"

func newRandomCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:     "random",
		Aliases: []string{"rand"},
		Short:   "Pick a random font",
		Example: `  gfonts-pp-cli random
  gfonts-pp-cli random --category display`,
		Args: cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			args := []string(nil)
			if category != "" {
				args = []string{"--category", category}
			}
			cmdRandom(args)
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "only pick from this category")
	return cmd
}
