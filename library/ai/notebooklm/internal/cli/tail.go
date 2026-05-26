package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tail",
		Short: "Stream live changes by polling at regular intervals",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("tail: not implemented")
		},
	}
}
