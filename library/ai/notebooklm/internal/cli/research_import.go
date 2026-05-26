package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResearchImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <notebook-id> <task-id>",
		Short: "Import research results as notebook sources",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("research", "import", args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
