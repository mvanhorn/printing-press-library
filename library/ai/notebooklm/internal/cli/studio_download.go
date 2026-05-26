package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStudioDownloadCmd() *cobra.Command {
	var artifactType, artifactID, output string
	cmd := &cobra.Command{
		Use:   "download <notebook-id>",
		Short: "Download a completed artifact to a local file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			if artifactType == "" {
				return fmt.Errorf("--type is required")
			}
			nlmArgs := []string{"download", artifactType, args[0]}
			if artifactID != "" {
				nlmArgs = append(nlmArgs, "--id", artifactID)
			}
			if output != "" {
				nlmArgs = append(nlmArgs, "--output", output)
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVarP(&artifactType, "type", "t", "", "Artifact type (audio, video, report, mind-map, slide-deck, infographic, data-table)")
	cmd.Flags().StringVar(&artifactID, "id", "", "Specific artifact ID to download")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	return cmd
}
