// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `models` — print the CLI-value -> model-key map for generation and
// remaster. Pure local; no network and no auth required. Read-only.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSunoModelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "models",
		Short:       "List available model names and their wire keys (generate + remaster)",
		Long:        "Print the mapping from CLI model values (e.g. v5.5) to Suno wire model keys (mv) for both generation and remaster. Works offline with no auth.",
		Example:     "  suno-pp-cli models\n  suno-pp-cli models --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				generate := make([]map[string]string, 0, len(sunoGenerateModelOrder))
				for _, v := range sunoGenerateModelOrder {
					generate = append(generate, map[string]string{"model": v, "mv": sunoGenerateModels[v]})
				}
				remaster := make([]map[string]string, 0, len(sunoRemasterModelOrder))
				for _, v := range sunoRemasterModelOrder {
					remaster = append(remaster, map[string]string{"model": v, "mv": sunoRemasterModels[v]})
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"default":  defaultGenerateModel,
					"generate": generate,
					"remaster": remaster,
				}, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Generation models (--model -> mv):")
			for _, v := range sunoGenerateModelOrder {
				def := ""
				if v == defaultGenerateModel {
					def = "  (default)"
				}
				fmt.Fprintf(w, "  %-6s -> %s%s\n", v, sunoGenerateModels[v], def)
			}
			fmt.Fprintln(w, "\nRemaster models (--model -> mv):")
			for _, v := range sunoRemasterModelOrder {
				fmt.Fprintf(w, "  %-6s -> %s\n", v, sunoRemasterModels[v])
			}
			return nil
		},
	}
	return cmd
}
