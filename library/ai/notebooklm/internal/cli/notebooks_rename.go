package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotebooksRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <notebook-id> <new-title>",
		Short: "Rename a notebook",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("notebook", "rename", args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
