// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTailCmd(flags *rootFlags) *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:         "tail [input.pdf]",
		Short:       "Show the last extracted text lines from a local PDF.",
		Example:     "  ihatepdf-cv-pp-cli tail report.pdf --lines 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "input=testdata/fixture.pdf"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("input PDF path is required"))
			}
			if lines < 1 || lines > 1000 {
				return usageErr(fmt.Errorf("--lines must be between 1 and 1000"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tail extracted PDF text")
			}
			b, err := readFile(args[0])
			if err != nil {
				return err
			}
			text := extractLiteralText(b)
			if text == "" {
				text = string(b)
			}
			rows := strings.Split(strings.TrimRight(text, "\n"), "\n")
			if len(rows) > lines {
				rows = rows[len(rows)-lines:]
			}
			return emitLocal(cmd, flags, map[string]any{"path": args[0], "lines": rows, "count": len(rows), "source": "local-file"})
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 20, "number of extracted text lines to return")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newTailCmd(flags)) })
}
