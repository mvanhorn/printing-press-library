package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotebooksCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new notebook",
		Example: `  notebooklm-pp-cli notebooks create "My Research"`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("notebook", "create", args[0])
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
