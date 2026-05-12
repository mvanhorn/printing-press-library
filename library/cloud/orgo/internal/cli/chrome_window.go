// Hand-authored: chrome window subcommand (resize).
package cli

import (
	"github.com/spf13/cobra"
)

func newChromeResizeCmd(flags *rootFlags) *cobra.Command {
	var width, height int
	cmd := &cobra.Command{
		Use:     "resize <id>",
		Short:   "Resize the Chrome viewport. Useful for responsive testing or wider screenshots.",
		Example: `  orgo-pp-cli chrome resize <id> --width 1920 --height 1080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"width": width, "height": height}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/resize", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().IntVar(&width, "width", 0, "Viewport width in pixels (required)")
	cmd.Flags().IntVar(&height, "height", 0, "Viewport height in pixels (required)")
	_ = cmd.MarkFlagRequired("width")
	_ = cmd.MarkFlagRequired("height")
	return cmd
}
