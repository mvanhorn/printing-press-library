// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newNovelFingerprintCmd(flags *rootFlags) *cobra.Command {
	var stdin bool
	cmd := &cobra.Command{Use: "fingerprint [files...]", Short: "Fingerprint local files; returns labeled SHA-256, SHA-1, and MD5 hashes.", Example: "  ihatepdf-cv-pp-cli fingerprint report.pdf --json\n  echo artifact | ihatepdf-cv-pp-cli fingerprint --stdin --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "input=testdata/fixture.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "fingerprint files")
		}
		if stdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			return emitLocal(cmd, flags, map[string]any{"files": []fileHash{hashBytes("<stdin>", b)}, "count": 1})
		}
		if len(args) == 0 {
			return usageErr(fmt.Errorf("provide at least one file or use --stdin"))
		}
		results := make([]fileHash, 0, len(args))
		for _, p := range args {
			// #nosec G304 -- fingerprint paths are explicit local CLI inputs.
			b, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("read %s: %w", p, err)
			}
			results = append(results, hashBytes(p, b))
		}
		return emitLocal(cmd, flags, map[string]any{"files": results, "count": len(results)})
	}}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "fingerprint bytes read from stdin")
	return cmd
}
