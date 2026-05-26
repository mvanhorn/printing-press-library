package cli

import "github.com/spf13/cobra"

func newStudioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "studio",
		Short: "Manage studio artifacts (audio, video, reports, etc.)",
	}
	cmd.AddCommand(newStudioCreateCmd())
	cmd.AddCommand(newStudioStatusCmd())
	cmd.AddCommand(newStudioDownloadCmd())
	return cmd
}
