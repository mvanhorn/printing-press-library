// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newImportCmd(flags *rootFlags) *cobra.Command {
	var inputFile string
	var dryRun bool
	var batchSize int

	cmd := &cobra.Command{
		Use:   "import <resource>",
		Short: "Import is not supported (Creative Fabrica catalog is search-only)",
		Long: `Creative Fabrica's public catalog is a search-only Algolia index.

There are no create/upsert endpoints, so JSONL import cannot write products.
Use find, free, pod, or products to search the catalog instead.`,
		Example: `  creativefabrica-pp-cli find "watercolor" --limit 10`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("import is not supported: Creative Fabrica exposes a search-only public catalog, not create/upsert endpoints")
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSONL file path (unused; import is not supported)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview import without sending requests (unused; import is not supported)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 1, "Records per batch (unused; import is not supported)")

	return cmd
}
