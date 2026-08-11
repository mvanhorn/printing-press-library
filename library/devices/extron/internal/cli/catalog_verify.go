// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: download integrity — compare local PDF sizes against the
// download ledger to flag truncated or mismatched downloads.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// pp:data-source local

type verifyRow struct {
	File         string `json:"file"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	ExpectedSize int64  `json:"expected_size"`
	ActualSize   int64  `json:"actual_size,omitempty"`
	Issue        string `json:"issue,omitempty"`
}

func newNovelCatalogVerifyCmd(flags *rootFlags) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:     "verify",
		Short:   "Compare local PDF sizes and revisions against the download ledger to flag truncated or mismatched downloads",
		Long:    "Check every entry in the download ledger (--dir/.extron-downloads.json): files that went missing, and files whose on-disk size differs from the size recorded at download time. Use this command after a batch download or sync to confirm every PDF landed complete. Downloads are the flaky leg of the pipeline (the site WAF intermittently resets connections), so integrity checks catch truncated files.",
		Example: "  extron-pp-cli catalog verify --dir ./docs --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify downloads")
			}
			if dir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dir is required (the folder holding downloaded docs)"))
			}
			recs, err := readLedgerLocked(dir)
			if err != nil {
				return err
			}
			rows := make([]verifyRow, 0, len(recs))
			for _, r := range recs {
				row := verifyRow{
					File:         r.File,
					Title:        r.Title,
					ExpectedSize: r.SizeBytes,
				}
				full, jerr := resolveLedgerPath(dir, r.File)
				if jerr != nil {
					row.Status = "error"
					row.Issue = jerr.Error()
					rows = append(rows, row)
					continue
				}
				fi, statErr := os.Stat(full)
				switch {
				case os.IsNotExist(statErr):
					row.Status = "missing"
					row.Issue = "file not found on disk"
				case statErr != nil:
					row.Status = "error"
					row.Issue = statErr.Error()
				case fi.Size() != r.SizeBytes:
					row.Status = "mismatch"
					row.ActualSize = fi.Size()
					row.Issue = fmt.Sprintf("size mismatch: expected %d bytes, found %d", r.SizeBytes, fi.Size())
				default:
					row.Status = "ok"
					row.ActualSize = fi.Size()
				}
				rows = append(rows, row)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No downloads recorded in the ledger. Run 'literature download' first.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"file":   r.File,
					"title":  r.Title,
					"status": r.Status,
					"issue":  r.Issue,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "directory holding downloaded docs and the download ledger (required)")
	return cmd
}
