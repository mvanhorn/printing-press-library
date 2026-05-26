package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSourcesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <source-id>",
		Short: "Delete a source from a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			extra := []string{}
			if flags.yes {
				extra = append(extra, "--confirm")
			}
			out, err := r.Run(append([]string{"source", "delete", args[0]}, extra...)...)
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}
