package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStudioStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <notebook-id>",
		Short: "Check generation status of studio artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("studio", "status", args[0], "--json")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
