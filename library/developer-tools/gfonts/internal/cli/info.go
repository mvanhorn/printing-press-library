package cli

import "github.com/spf13/cobra"

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info <font>",
		Aliases: []string{"show"},
		Short:   "Show detailed font information",
		Example: `  gfonts-pp-cli info "Inter"
  gfonts-pp-cli info "Playfair Display"`,
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cmdInfo(args)
		},
	}
}
