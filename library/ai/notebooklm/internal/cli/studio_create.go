package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStudioCreateCmd() *cobra.Command {
	var artifactType, format, language string
	cmd := &cobra.Command{
		Use:   "create <notebook-id>",
		Short: "Create a studio artifact",
		Args:  cobra.ExactArgs(1),
		Long: `Create a studio artifact from a notebook's sources.
Supported types: audio, video, report, slides, infographic, mindmap, quiz, flashcards, data-table`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			if artifactType == "" {
				return fmt.Errorf("--type is required (audio, video, report, slides, infographic, mindmap, quiz, flashcards, data-table)")
			}
			nlmArgs := []string{artifactType, "create", args[0]}
			if format != "" {
				nlmArgs = append(nlmArgs, "--format", format)
			}
			if language != "" {
				nlmArgs = append(nlmArgs, "--language", language)
			}
			if flags.yes {
				nlmArgs = append(nlmArgs, "--confirm")
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVarP(&artifactType, "type", "t", "", "Artifact type (audio, video, report, slides, infographic, mindmap, quiz, flashcards, data-table)")
	cmd.Flags().StringVar(&format, "format", "", "Artifact format variant")
	cmd.Flags().StringVar(&language, "language", "", "BCP-47 language code (en, es, fr, de, ja)")
	return cmd
}
