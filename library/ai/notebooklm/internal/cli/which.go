package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWhichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which",
		Short: "Find the command that implements a capability",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("which: not implemented")
		},
	}
}
