// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored literature commands: list, get, download — plus registration
// of every literature novel child under the generated `literature` parent.

package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

func init() {
	// Wire the hand-authored literature children under the generated
	// `literature` parent (promoted endpoint command). The generator already
	// attaches recent/family/updates/rack; list/get/download are added here.
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		litCmd, _, err := root.Find([]string{"literature"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(litCmd, newNovelLiteratureListCmd(flags))
		addNovelCommandIfAbsent(litCmd, newNovelLiteratureGetCmd(flags))
		addNovelCommandIfAbsent(litCmd, newNovelLiteratureDownloadCmd(flags))
	})
}

// pp:data-source local

func newNovelLiteratureListCmd(flags *rootFlags) *cobra.Command {
	var dbPath, category, letter string
	var limit int

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List literature from the local catalog, filterable by category and letter",
		Example: "  extron-pp-cli literature list --category manual --letter m\n  extron-pp-cli literature list --category brochure --limit 25 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list literature")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = resolveCatalogDB(flags, dbPath)
			db, ok := openCatalogReadOnly(cmd, dbPath)
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), make([]extron.Doc, 0), flags)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, catalogResource)
			hintIfCatalogIncomplete(cmd, db)

			docs, err := loadCatalogDocs(ctx, db)
			if err != nil {
				return err
			}
			rows := make([]extron.Doc, 0, len(docs))
			for _, d := range docs {
				if !matchCategory(d, category) || !matchLetter(d, letter) {
					continue
				}
				rows = append(rows, d)
			}
			orderDocsByTitle(rows)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No literature matches the filters. Run 'extron-pp-cli catalog sync' first if the catalog is empty.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), docsToTable(rows))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&category, "category", "", "filter by category (brochure, manual, declaration, design guide, product guide, revit bim)")
	cmd.Flags().StringVar(&letter, "letter", "", "filter by alphabetical index letter (A-Z, or 0 for non-alpha)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = no limit)")
	return cmd
}

// pp:data-source auto

func newNovelLiteratureGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:     "get [model]",
		Short:   "Resolve a product or document name to its official Extron literature",
		Long:    "Resolve a product or document name (e.g. \"MAV Plus Series\", \"DTP2 T 211 4K\") to its official Extron literature from the local catalog. Use this command for a single model's doc list. Do NOT use it for whole-family browsing; use 'literature family' instead.",
		Example: "  extron-pp-cli literature get \"MAV Plus Series\" --json\n  extron-pp-cli literature get \"12G HD-SDI\" --limit 5",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "<model>=MAV Plus Series",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "get literature")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a model or document name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			query := strings.Join(args, " ")
			docs, err := catalogOrLive(ctx, cmd, flags, dbPath, query)
			if err != nil {
				return err
			}
			rows := resolveDocs(docs, query, limit)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No literature matches that name. Run 'extron-pp-cli catalog sync' if the catalog is empty, then retry.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), docsToTable(rows))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum matches to return")
	return cmd
}

// pp:data-source live

// dlResult is one download outcome row.
type dlResult struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
	File     string `json:"file"`
	Bytes    int64  `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
}

func newNovelLiteratureDownloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath, dir, category string
	var limit int

	cmd := &cobra.Command{
		Use:     "download [name-or-url]",
		Short:   "Download official Extron spec sheets and manuals as PDFs",
		Long:    "Download official Extron literature as PDFs. The argument is either a document URL or a product/document name resolved through the local catalog. Files are written to --dir/<category>/ using the upstream filename, and the download ledger (.extron-downloads.json) is updated so 'literature updates' and 'catalog verify' can track revisions and integrity.",
		Example: "  extron-pp-cli literature download \"MAV Plus Series\" --dir ./docs\n  extron-pp-cli literature download /download/files/userman/68-3006-50_B-12G_HD-SDI_101.pdf --dir ./docs --dry-run",
		Annotations: map[string]string{
			"pp:happy-args": "<name-or-url>=MAV Plus Series",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "download literature")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a document URL or name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dir == "" {
				dir = "."
			}

			targets, err := resolveDownloadTargets(ctx, cmd, flags, dbPath, args[0], category, limit)
			if err != nil {
				return err
			}
			client := extron.New()
			results := make([]dlResult, 0, len(targets))
			for _, d := range targets {
				res := dlResult{Title: d.Title, Category: d.Category, URL: d.URL}
				rel := downloadDest(dir, d)
				res.File = rel
				full, jerr := resolveLedgerPath(dir, rel)
				if jerr != nil {
					res.Error = jerr.Error()
					results = append(results, res)
					continue
				}
				if flags.dryRun {
					results = append(results, res)
					continue
				}
				n, err := client.Download(ctx, client.AbsoluteURL(d.URL), full)
				if err != nil {
					res.Error = err.Error()
				} else {
					res.Bytes = n
				}
				results = append(results, res)
			}
			if !flags.dryRun {
				if err := appendDownloadLedger(dir, targets, results); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			failed := 0
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "failed: %s -> %s (%s)\n", r.Title, r.File, r.Error)
					failed++
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "downloaded: %s -> %s\n", r.Title, r.File)
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d downloads failed", failed, len(results))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&dir, "dir", "", "download directory (default: current directory)")
	cmd.Flags().StringVar(&category, "category", "", "restrict resolution to a category")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum documents to download when resolving by name")
	return cmd
}

// resolveDownloadTargets turns a URL or name argument into concrete docs.
// URL arguments are used directly; names are resolved through the local
// catalog, with a live fallback (first-letter bucket + category page-2) when
// the catalog is empty or yields no match.
func resolveDownloadTargets(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, arg, category string, limit int) ([]extron.Doc, error) {
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "/download/") {
		title := filepath.Base(arg)
		if i := strings.Index(title, "-"); i > 0 {
			title = title[:i]
		}
		return []extron.Doc{{Title: title, Category: category, URL: arg}}, nil
	}
	dbPath = resolveCatalogDB(flags, dbPath)
	if db, ok := openCatalogReadOnly(cmd, dbPath); ok {
		docs, err := loadCatalogDocs(ctx, db)
		db.Close()
		if err != nil {
			return nil, err
		}
		if len(docs) > 0 {
			rows := resolveDocs(docs, arg, limit)
			rows = filterCategory(rows, category)
			if len(rows) > 0 {
				return rows, nil
			}
		}
	}
	// Live fallback: fetch the letter bucket of the query's first character
	// (plus each category's page-2) and resolve there.
	client := extron.New()
	letter := "All"
	for _, r := range arg {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letter = strings.ToUpper(string(r))
			break
		}
	}
	docs, refs, err := client.FetchIndex(ctx, letter)
	if err != nil {
		return nil, fmt.Errorf("catalog has no match and live fetch failed: %w", err)
	}
	for _, ref := range refs {
		paged, _, perr := client.FetchCategoryPage(ctx, letter, ref)
		if perr != nil {
			continue
		}
		docs = append(docs, paged...)
	}
	rows := resolveDocs(docs, arg, limit)
	rows = filterCategory(rows, category)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no literature matches %q (run 'extron-pp-cli catalog sync' for the full catalog)", arg)
	}
	return rows, nil
}

func filterCategory(rows []extron.Doc, category string) []extron.Doc {
	if category == "" {
		return rows
	}
	out := make([]extron.Doc, 0, len(rows))
	for _, d := range rows {
		if matchCategory(d, category) {
			out = append(out, d)
		}
	}
	return out
}

func downloadDest(dir string, d extron.Doc) string {
	base := filepath.Base(d.URL)
	if base == "." || base == "/" || base == "" {
		base = strings.ReplaceAll(strings.ToLower(d.Title), " ", "_") + ".pdf"
	}
	// Ledger records the path RELATIVE to --dir so later joins cannot
	// double-prefix or escape the directory.
	return filepath.Join(safeSubdir(d.Category), base)
}

// safeSubdir turns a category name into a single safe path segment.
func safeSubdir(category string) string {
	sub := strings.ToLower(strings.TrimSpace(category))
	sub = strings.ReplaceAll(sub, " ", "-")
	sub = filepath.Base(sub)
	if sub == "." || sub == ".." || sub == "/" || sub == "" || strings.ContainsAny(sub, "/\\") {
		sub = "literature"
	}
	return sub
}

func orderDocsByTitle(docs []extron.Doc) {
	sortDocs := docs
	for i := 1; i < len(sortDocs); i++ {
		for j := i; j > 0 && strings.ToLower(sortDocs[j].Title) < strings.ToLower(sortDocs[j-1].Title); j-- {
			sortDocs[j], sortDocs[j-1] = sortDocs[j-1], sortDocs[j]
		}
	}
}

// appendDownloadLedger records completed downloads (and their catalog rev/size
// metadata) into the --dir ledger so updates/verify can track drift.
func appendDownloadLedger(dir string, targets []extron.Doc, results []dlResult) error {
	recs := make([]downloadRecord, 0, len(results))
	for i, r := range results {
		if r.Error != "" {
			continue
		}
		doc := extron.Doc{}
		if i < len(targets) {
			doc = targets[i]
		}
		recs = append(recs, newDownloadRecord(doc, r.File, r.Bytes))
	}
	return upsertLedgerRecords(dir, recs)
}
