package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotesCreateCmd() *cobra.Command {
	var title, content string
	cmd := &cobra.Command{
		Use:   "create <notebook-id>",
		Short: "Create a note in a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"note", "create", args[0]}
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
	cmd.Flags().StringVar(&title, "title", "", "Note title")
	cmd.Flags().StringVar(&content, "content", "", "Note content")
	return cmd
}
