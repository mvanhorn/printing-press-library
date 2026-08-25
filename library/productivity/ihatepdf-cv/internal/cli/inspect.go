// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func newNovelInspectCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "inspect [input.pdf]", Short: "Inspect local PDF structure and metadata; returns page count, hints, and hashes.", Example: "  ihatepdf-cv-pp-cli inspect report.pdf --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "input=testdata/fixture.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "inspect PDF")
		}
		if len(args) < 1 {
			return usageErr(fmt.Errorf("input PDF path is required"))
		}
		b, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", args[0], err)
		}
		pages, pageErr := api.PageCountFile(args[0])
		validErr := api.ValidateFile(args[0], model.NewDefaultConfiguration())
		hints := make([]string, 0)
		for _, key := range []string{"/Author", "/Creator", "/Producer", "/Title", "/Subject"} {
			if strings.Contains(string(b), key) {
				hints = append(hints, key)
			}
		}
		r := inspectResult{Path: args[0], Size: int64(len(b)), Pages: pages, Valid: pageErr == nil && validErr == nil, MetadataHints: hints, Hashes: hashBytes(args[0], b)}
		if pageErr != nil && validErr != nil {
			return fmt.Errorf("inspect PDF %s: %w", args[0], pageErr)
		}
		return emitLocal(cmd, flags, r)
	}}
	return cmd
}
