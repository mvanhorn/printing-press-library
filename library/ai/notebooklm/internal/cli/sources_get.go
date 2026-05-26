package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <source-id>",
		Short: "Get source metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("source", "get", args[0], "--json")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
