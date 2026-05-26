package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <notebook-id>",
		Short: "List notes in a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("note", "list", args[0], "--json")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
