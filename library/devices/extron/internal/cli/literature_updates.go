// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: revision drift check — which downloaded docs have a newer
// revision available upstream.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

// pp:data-source local

type updateRow struct {
	File       string `json:"file"`
	Title      string `json:"title"`
	Category   string `json:"category"`
	URL        string `json:"url"`
	LocalRev   string `json:"local_rev"`
	CatalogRev string `json:"catalog_rev"`
	Update     bool   `json:"update_available"`
	Missing    bool   `json:"file_missing,omitempty"`
}

func newNovelLiteratureUpdatesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, dir string
	var download bool
	var limit int

	cmd := &cobra.Command{
		Use:     "updates",
		Short:   "See which downloaded spec sheets and manuals have a newer revision available upstream",
		Long:    "Compare the download ledger (--dir/.extron-downloads.json) against the local catalog's revision column and list docs where a newer revision exists. Use this command to find which downloaded docs need re-downloading. Do NOT use it to browse brand-new library arrivals; use 'literature recent' instead. Pass --download to re-fetch the superseded docs.",
		Example: "  extron-pp-cli literature updates --dir ./docs --json\n  extron-pp-cli literature updates --dir ./docs --download",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "check literature updates")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dir is required (the folder holding downloaded docs)"))
			}
			dbPath = resolveCatalogDB(flags, dbPath)
			db, ok := openCatalogReadOnly(cmd, dbPath)
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), make([]updateRow, 0), flags)
			}
			defer db.Close()
			hintIfCatalogIncomplete(cmd, db)

			docs, err := loadCatalogDocs(ctx, db)
			if err != nil {
				return err
			}
			byURL := make(map[string]extron.Doc, len(docs))
			for _, d := range docs {
				byURL[d.URL] = d
			}
			recs, err := readLedgerLocked(dir)
			if err != nil {
				return err
			}
			rows := make([]updateRow, 0, len(recs))
			for _, r := range recs {
				row := updateRow{
					File:       r.File,
					Title:      r.Title,
					Category:   r.Category,
					URL:        r.URL,
					LocalRev:   r.Rev,
					CatalogRev: r.Rev,
				}
				full, jerr := resolveLedgerPath(dir, r.File)
				if jerr != nil {
					row.Missing = true
					row.Update = true
				} else if _, statErr := os.Stat(full); os.IsNotExist(statErr) {
					row.Missing = true
					row.Update = true
				}
				if c, ok := byURL[r.URL]; ok {
					row.CatalogRev = c.Rev
					row.Category = c.Category
					if c.Rev != "" && r.Rev != "" && !revsEqual(c.Rev, r.Rev) {
						row.Update = true
					}
				}
				if row.Update {
					rows = append(rows, row)
					if limit > 0 && len(rows) >= limit {
						break
					}
				}
			}
			if download {
				if err := reDownloadUpdates(ctx, rows, dir); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All downloaded docs are current (or the ledger is empty).")
				return nil
			}
			rowsMap := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				rowsMap = append(rowsMap, map[string]any{
					"file":         r.File,
					"title":        r.Title,
					"local_rev":    r.LocalRev,
					"catalog_rev":  r.CatalogRev,
					"update":       r.Update,
					"file_missing": r.Missing,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rowsMap)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&dir, "dir", "", "directory holding downloaded docs and the download ledger (required)")
	cmd.Flags().BoolVar(&download, "download", false, "re-download the superseded docs after listing")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = no limit)")
	return cmd
}

// revsEqual compares revision labels leniently ("Rev E" == "E", "B" == "B",
// "REV C1" == "C1").
func revsEqual(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToUpper(strings.TrimSpace(s))
		s = strings.TrimPrefix(s, "REV ")
		s = strings.TrimPrefix(s, "REV_")
		s = strings.TrimPrefix(s, "REV")
		return s
	}
	return norm(a) == norm(b)
}

func reDownloadUpdates(ctx context.Context, rows []updateRow, dir string) error {
	client := extron.New()
	for _, r := range rows {
		full, err := resolveLedgerPath(dir, r.File)
		if err != nil {
			return fmt.Errorf("re-downloading %s: %w", r.Title, err)
		}
		n, err := client.Download(ctx, client.AbsoluteURL(r.URL), full)
		if err != nil {
			return fmt.Errorf("re-downloading %s: %w", r.Title, err)
		}
		rec := downloadRecord{
			File:         r.File,
			Title:        r.Title,
			Category:     r.Category,
			URL:          r.URL,
			Rev:          r.CatalogRev,
			SizeBytes:    n,
			DownloadedAt: nowRFC3339(),
		}
		if err := upsertLedgerRecords(dir, []downloadRecord{rec}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "re-downloaded: %s -> %s\n", r.Title, r.File)
	}
	return nil
}
