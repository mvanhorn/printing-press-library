// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:         "export [files...]",
		Short:       "Export a local PDF inspection manifest with pages, metadata, and hashes.",
		Example:     "  ihatepdf-cv-pp-cli export reports/*.pdf --output manifest.json --agent",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:happy-args": "first=testdata/fixture.pdf;--output=verify-manifest.json"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("provide at least one PDF path"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export PDF inspection manifest")
			}
			if err := refuseOverwrite(output); err != nil {
				return usageErr(err)
			}
			entries := make([]inspectResult, 0, len(args))
			for _, path := range args {
				b, err := readFile(path)
				if err != nil {
					return err
				}
				pages, pageErr := api.PageCountFile(path)
				validErr := api.ValidateFile(path, model.NewDefaultConfiguration())
				entries = append(entries, inspectResult{Path: path, Size: int64(len(b)), Pages: pages, Valid: pageErr == nil && validErr == nil, Hashes: hashBytes(path, b)})
			}
			if err := writeJSONFile(output, map[string]any{"files": entries, "count": len(entries), "source": "local-file"}); err != nil {
				return err
			}
			return emitLocal(cmd, flags, map[string]any{"operation": "export", "output": output, "count": len(entries), "source": "local-file"})
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "JSON manifest output path")
	return cmd
}

func writeJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newExportCmd(flags)) })
}
