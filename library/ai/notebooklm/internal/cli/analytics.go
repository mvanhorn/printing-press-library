package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAnalyticsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analytics",
		Short: "Run analytics queries on locally synced data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("analytics: not implemented")
		},
	}
}
