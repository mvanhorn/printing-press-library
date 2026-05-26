package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWorkflowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "workflow",
		Short: "Compound workflows combining multiple operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("workflow: not implemented")
		},
	}
}
