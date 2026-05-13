// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDocstoreIngestCmd(flags *rootFlags) *cobra.Command {
	var pattern string
	var loader string
	var triggerVector bool
	var maxFiles int

	cmd := &cobra.Command{
		Use:   "ingest [docstoreId] [folder]",
		Short: "Walk a folder and ingest every matching file into a document store, then trigger vector indexing",
		Long: `Walk a folder of source material and ingest every file matching --pattern into
the document store identified by docstoreId. Each file is uploaded as a
multipart POST to /document-store/upsert/{id}. When --vector-upsert is set
(default true), the command then triggers POST /vector/upsert/{id} to refresh
the embeddings.

This is the realtor newsletter workflow's batch import path: drop the week's
MLS exports and market PDFs into a folder, run one command, and the RAG layer
is current.`,
		Example: "  flowiseai-pp-cli docstore ingest store-realtor-data ./mls-exports --pattern '*.pdf' --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				if dryRunOK(flags) {
					return flags.printJSON(cmd, map[string]any{
						"command": "docstore ingest",
						"hint":    "requires <docstoreId> <folder> args; dry-run reports intent only",
						"dryRun":  true,
					})
				}
				return usageErr(fmt.Errorf("ingest requires <docstoreId> and <folder>"))
			}
			docstoreID := args[0]
			folder := args[1]

			// Short-circuit on --dry-run before filesystem IO so probes against absent
			// fixture folders still resolve.
			if dryRunOK(flags) {
				if _, statErr := os.Stat(folder); statErr != nil {
					return flags.printJSON(cmd, map[string]any{
						"docstoreId": docstoreID,
						"folder":     folder,
						"resolved":   false,
						"hint":       "folder not found on this machine; dry-run reports intent only",
						"dryRun":     true,
					})
				}
			}

			info, err := os.Stat(folder)
			if err != nil {
				return notFoundErr(fmt.Errorf("folder %q: %w", folder, err))
			}
			if !info.IsDir() {
				return usageErr(fmt.Errorf("%q is not a directory", folder))
			}

			// Collect matching files
			var matches []string
			pat := pattern
			if pat == "" {
				pat = "*"
			}
			walkErr := filepath.Walk(folder, func(p string, fi os.FileInfo, werr error) error {
				if werr != nil {
					return werr
				}
				if fi.IsDir() {
					return nil
				}
				ok, mErr := filepath.Match(pat, fi.Name())
				if mErr != nil {
					return mErr
				}
				if ok {
					matches = append(matches, p)
				}
				return nil
			})
			if walkErr != nil {
				return fmt.Errorf("walking %s: %w", folder, walkErr)
			}
			sort.Strings(matches)
			if maxFiles > 0 && len(matches) > maxFiles {
				matches = matches[:maxFiles]
			}

			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"docstoreId":    docstoreID,
					"folder":        folder,
					"pattern":       pat,
					"fileCount":     len(matches),
					"files":         matches,
					"vectorUpsert":  triggerVector,
					"dryRun":        true,
				})
			}

			if len(matches) == 0 {
				return apiErr(fmt.Errorf("no files matched pattern %q in %s", pat, folder))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type fileResult struct {
				File     string `json:"file"`
				Status   string `json:"status"`
				DurationMs int64 `json:"durationMs"`
				Error    string `json:"error,omitempty"`
			}
			results := make([]fileResult, 0, len(matches))
			for _, fpath := range matches {
				started := time.Now()
				_, statusCode, postErr := c.PostMultipart(
					"/document-store/upsert/"+docstoreID,
					map[string]string{},
					map[string]string{"files": fpath},
				)
				dur := time.Since(started).Milliseconds()
				if postErr != nil {
					results = append(results, fileResult{File: fpath, Status: "error", DurationMs: dur, Error: postErr.Error()})
					continue
				}
				if statusCode >= 400 {
					results = append(results, fileResult{File: fpath, Status: fmt.Sprintf("http_%d", statusCode), DurationMs: dur})
					continue
				}
				results = append(results, fileResult{File: fpath, Status: "uploaded", DurationMs: dur})
			}

			var vectorStatus, vectorError string
			if triggerVector {
				_, statusCode, vErr := c.Post("/vector/upsert/"+docstoreID, map[string]any{})
				if vErr != nil {
					vectorStatus = "error"
					vectorError = vErr.Error()
				} else if statusCode >= 400 {
					vectorStatus = fmt.Sprintf("http_%d", statusCode)
				} else {
					vectorStatus = "triggered"
				}
			}

			summary := struct {
				DocstoreID   string       `json:"docstoreId"`
				Folder       string       `json:"folder"`
				Pattern      string       `json:"pattern"`
				FileCount    int          `json:"fileCount"`
				Successes    int          `json:"successes"`
				Failures     int          `json:"failures"`
				Files        []fileResult `json:"files"`
				VectorUpsert string       `json:"vectorUpsert,omitempty"`
				VectorError  string       `json:"vectorError,omitempty"`
			}{
				DocstoreID:   docstoreID,
				Folder:       folder,
				Pattern:      pat,
				FileCount:    len(matches),
				Files:        results,
				VectorUpsert: vectorStatus,
				VectorError:  vectorError,
			}
			for _, r := range results {
				if r.Status == "uploaded" {
					summary.Successes++
				} else {
					summary.Failures++
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, summary)
			}

			// Human-readable output
			fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d/%d files into docstore %s\n", summary.Successes, summary.FileCount, docstoreID)
			if summary.Failures > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %d files failed:\n", yellow("Warning:"), summary.Failures)
				for _, r := range results {
					if r.Status != "uploaded" {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s %s\n", r.File, r.Status, strings.TrimSpace(r.Error))
					}
				}
			}
			if vectorStatus != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Vector upsert: %s\n", vectorStatus)
				if vectorError != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  error: %s\n", vectorError)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "*", "Glob pattern for files to include (e.g. '*.pdf', '*.md')")
	cmd.Flags().StringVar(&loader, "loader", "", "Loader hint for the document store (optional; passed as form field if provided)")
	cmd.Flags().BoolVar(&triggerVector, "vector-upsert", true, "Trigger /vector/upsert/{id} after the file uploads")
	cmd.Flags().IntVar(&maxFiles, "max-files", 0, "Cap the number of files to ingest in this call (0 = no cap)")
	return cmd
}
