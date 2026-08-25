// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/internal/store"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/spf13/cobra"
)

func newCatalogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "catalog", Short: "Index and search a local PDF catalog without uploading documents.", Annotations: map[string]string{"mcp:read-only": "false"}}
	cmd.AddCommand(newCatalogIndexCmd(flags), newCatalogSearchCmd(flags), newCatalogStatusCmd(flags))
	return cmd
}

func catalogDBPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	return defaultDBPath("ihatepdf-cv-pp-cli")
}

func newCatalogIndexCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var recursive bool
	cmd := &cobra.Command{
		Use:   "index [paths...]",
		Short: "Index PDF files by path, page count, hash, and extracted text.",
		Example: `  ihatepdf-cv-pp-cli catalog index ./reports --recursive --agent
  ihatepdf-cv-pp-cli catalog index report.pdf --db ./catalog.db --json`,
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:happy-args": "path=testdata"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("provide at least one PDF path or directory"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "index local PDF catalog")
			}
			db, err := store.OpenWithContext(cmd.Context(), catalogDBPath(dbPath))
			if err != nil {
				return fmt.Errorf("open catalog: %w", err)
			}
			defer db.Close()
			indexed := 0
			for _, root := range args {
				walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if entry.IsDir() {
						if path != root && !recursive {
							return filepath.SkipDir
						}
						return nil
					}
					if strings.ToLower(filepath.Ext(path)) != ".pdf" {
						return nil
					}
					b, readErr := readFile(path)
					if readErr != nil {
						return readErr
					}
					info, statErr := os.Stat(path)
					if statErr != nil {
						return statErr
					}
					pages, _ := api.PageCountFile(path)
					hash := hashBytes(path, b)
					record := store.PDFRecord{ID: hash.SHA256, Path: path, Size: info.Size(), Pages: pages, SHA256: hash.SHA256, Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"), Text: extractLiteralText(b)}
					if err := db.UpsertPDF(record); err != nil {
						return fmt.Errorf("index %s: %w", path, err)
					}
					indexed++
					return nil
				})
				if walkErr != nil {
					return walkErr
				}
			}
			if err := db.SaveSyncState("pdf_catalog", "", indexed); err != nil {
				return fmt.Errorf("save catalog state: %w", err)
			}
			return emitLocal(cmd, flags, map[string]any{"operation": "catalog-index", "indexed": indexed, "db": catalogDBPath(dbPath), "source": "local-file"})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite catalog path")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "walk nested directories")
	return cmd
}

func newCatalogSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Search indexed PDF paths and extracted text from the local catalog.",
		Example:     "  ihatepdf-cv-pp-cli catalog search invoice --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("query is required"))
			}
			if limit < 1 || limit > 1000 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search local PDF catalog")
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), catalogDBPath(dbPath))
			if err != nil {
				return fmt.Errorf("open catalog: %w", err)
			}
			defer db.Close()
			results, err := db.SearchPDF(args[0], limit)
			if err != nil {
				return fmt.Errorf("search catalog: %w", err)
			}
			return emitLocal(cmd, flags, map[string]any{"query": args[0], "results": results, "count": len(results), "source": "local-file"})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite catalog path")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum matches")
	return cmd
}

func newCatalogStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Report indexed PDF count and last local catalog update.",
		Example:     "  ihatepdf-cv-pp-cli catalog status --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inspect local PDF catalog status")
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), catalogDBPath(dbPath))
			if err != nil {
				return fmt.Errorf("open catalog: %w", err)
			}
			defer db.Close()
			_, syncedAt, count, err := db.GetSyncState("pdf_catalog")
			if err != nil {
				return fmt.Errorf("read catalog status: %w", err)
			}
			return emitLocal(cmd, flags, map[string]any{"resource": "pdf_catalog", "count": count, "last_indexed": syncedAt, "source": "local-file"})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite catalog path")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newCatalogCmd(flags)) })
}
