package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotesUpdateCmd() *cobra.Command {
	var title, content string
	cmd := &cobra.Command{
		Use:   "update <note-id>",
		Short: "Update a note's content or title",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"note", "update", args[0]}
			if title != "" {
				nlmArgs = append(nlmArgs, "--title", title)
			}
			if content != "" {
				nlmArgs = append(nlmArgs, "--content", content)
			}
			out, err := r.Run(nlmArgs...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&content, "content", "", "New content")
	return cmd
}
