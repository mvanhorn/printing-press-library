package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import",
		Short: "Import data from JSONL file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("import: not implemented")
		},
	}
}
