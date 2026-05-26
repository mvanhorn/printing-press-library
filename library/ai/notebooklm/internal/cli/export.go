package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export data to JSONL for backup or migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("export: not implemented")
		},
	}
}
