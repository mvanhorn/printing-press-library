package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <source-id>",
		Short: "AI summary and keyword chips for a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			out, err := r.Run("source", "describe", args[0], "--json")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
