package cli

import "github.com/spf13/cobra"

func newTrendingCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "trending [limit]",
		Aliases: []string{"popular"},
		Short:   "Show popular and trending fonts",
		Example: `  gfonts-pp-cli trending
  gfonts-pp-cli trending 25`,
		Args: cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cmdTrending(args)
		},
	}
}
