package cli

import "github.com/spf13/cobra"

func newDownloadCmd() *cobra.Command {
	var variant string
	var output string
	var show bool
	cmd := &cobra.Command{
		Use:     "download <font>",
		Aliases: []string{"get", "dl"},
		Short:   "Download font files",
		Example: `  gfonts-pp-cli download "Inter" --variant regular --output ./my-fonts
  gfonts-pp-cli download "Playfair Display" --show`,
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			legacyArgs := append([]string(nil), args...)
			if variant != "" {
				legacyArgs = append(legacyArgs, "--variant", variant)
			}
			if output != "" {
				legacyArgs = append(legacyArgs, "--output", output)
			}
			if show {
				legacyArgs = append(legacyArgs, "--show")
			}
			cmdDownload(legacyArgs)
		},
	}
	cmd.Flags().StringVarP(&variant, "variant", "v", "", "download a specific variant")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output directory")
	cmd.Flags().BoolVarP(&show, "show", "s", false, "show URLs without downloading")
	return cmd
}
