package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFeedbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feedback",
		Short: "Record feedback about this CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("feedback: not implemented")
		},
	}
}
