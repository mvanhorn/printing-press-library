package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check NotebookLM authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("login", "--check")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
