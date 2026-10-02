package cli

import "github.com/spf13/cobra"

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search fonts by name, category, or designer",
		Example: `  gfonts-pp-cli search "Inter"
  gfonts-pp-cli search "sans-serif"`,
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cmdSearch(args)
		},
	}
}
