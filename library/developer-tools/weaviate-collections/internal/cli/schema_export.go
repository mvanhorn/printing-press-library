// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: schema export (see schema_import.go for the counterpart).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type schemaBundle struct {
	ExportedAt string           `json:"exported_at"`
	Classes    []map[string]any `json:"classes"`
}

// pp:data-source live
func newNovelSchemaExportCmd(flags *rootFlags) *cobra.Command {
	var flagOutput string
	var flagCollection string

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export every collection's config as one portable JSON bundle for backup or promotion to another environment.",
		Example: "  weaviate-collections-pp-cli schema export --output schema-bundle.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export collection config(s) to a bundle")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var classes []map[string]any
			if flagCollection != "" {
				cls, err := fetchOneClass(ctx, flags, flagCollection)
				if err != nil {
					return err
				}
				classes = []map[string]any{cls}
			} else {
				all, err := fetchAllClasses(ctx, flags)
				if err != nil {
					return err
				}
				classes = all
			}

			bundle := schemaBundle{
				ExportedAt: time.Now().UTC().Format(time.RFC3339),
				Classes:    classes,
			}
			payload, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return fmt.Errorf("encoding bundle: %w", err)
			}

			if flagOutput != "" && flagOutput != "-" {
				// 0o600: the bundle can contain internal collection config
				// (property names, vectorizer settings, module config); keep
				// it readable only by the invoking user, not world-readable.
				if err := os.WriteFile(flagOutput, payload, 0o600); err != nil {
					return fmt.Errorf("writing bundle to %s: %w", flagOutput, err)
				}
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "exported %d collection(s) to %s\n", len(classes), flagOutput)
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"output":      flagOutput,
					"num_classes": len(classes),
				}, flags)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(payload))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOutput, "output", "", "File to write the bundle to (default: print to stdout)")
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Export only this collection (default: all collections)")
	return cmd
}
