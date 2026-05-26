package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesAddCmd() *cobra.Command {
	var url, text, drive, file, title string
	var wait bool
	cmd := &cobra.Command{
		Use:   "add <notebook-id>",
		Short: "Add a source to a notebook",
		Example: `notebooklm-pp-cli sources add <notebook-id> --url https://example.com`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"source", "add", args[0]}
			if url != "" {
				nlmArgs = append(nlmArgs, "--url", url)
			}
			if text != "" {
				nlmArgs = append(nlmArgs, "--text", text)
				if title != "" {
					nlmArgs = append(nlmArgs, "--title", title)
				}
			}
			if drive != "" {
				nlmArgs = append(nlmArgs, "--drive", drive)
			}
			if file != "" {
				nlmArgs = append(nlmArgs, "--file", file)
			}
			if wait {
				nlmArgs = append(nlmArgs, "--wait")
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "URL to add as source")
	cmd.Flags().StringVar(&text, "text", "", "Text content to add as source")
	cmd.Flags().StringVar(&drive, "drive", "", "Google Drive document ID to add")
	cmd.Flags().StringVar(&file, "file", "", "Local file path to upload")
	cmd.Flags().StringVar(&title, "title", "", "Title for text source")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait until source is processed")
	return cmd
}
