// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/internal/store"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var outputFile string
	var resourceType string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export locally cached resources to a JSONL file for offline use",
		Long: `Export resources from the local SQLite store to a JSONL file.
Each line is a JSON object. Use 'sync' first to populate the local store.`,
		Example: `  # Export all feature flags to stdout
  posthog-pp-cli export --resource feature_flags

  # Export to a file
  posthog-pp-cli export --resource experiments --output experiments.jsonl

  # Export with a row limit
  posthog-pp-cli export --resource feature_flags --limit 500`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("posthog-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			items, err := s.List(resourceType, limit)
			if err != nil {
				return fmt.Errorf("listing %s: %w", resourceType, err)
			}

			var out *os.File
			if outputFile == "" || outputFile == "-" {
				out = os.Stdout
			} else {
				out, err = os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer out.Close()
			}

			enc := json.NewEncoder(out)
			for _, raw := range items {
				if err := enc.Encode(raw); err != nil {
					return err
				}
			}

			if outputFile != "" && outputFile != "-" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Exported %d %s records to %s\n", len(items), resourceType, outputFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&resourceType, "resource", "feature_flags", "Resource type to export (e.g. feature_flags, experiments, persons)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Output file path (default: stdout)")
	cmd.Flags().IntVar(&limit, "limit", 10000, "Maximum number of records to export from the local store")
	return cmd
}
