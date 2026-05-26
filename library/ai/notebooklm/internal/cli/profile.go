package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "Named sets of flags saved for reuse",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("profile: not implemented")
		},
	}
}
