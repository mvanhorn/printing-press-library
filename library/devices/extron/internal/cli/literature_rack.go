// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: BOM doc bundle — resolve every model in a rack BOM to its
// full official doc set, and optionally download it.

package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

// pp:data-source local

type rackRow struct {
	Model    string       `json:"model"`
	DocCount int          `json:"doc_count"`
	Docs     []extron.Doc `json:"docs,omitempty"`
	Missing  bool         `json:"missing"`
}

func newNovelLiteratureRackCmd(flags *rootFlags) *cobra.Command {
	var dbPath, bomPath, dir string
	var download bool
	var perModel int

	cmd := &cobra.Command{
		Use:     "rack",
		Short:   "Assemble the full official doc set for every model in a rack bill of materials",
		Long:    "Read a rack BOM (one model per line, or CSV with the model in the first column), resolve each model against the local catalog, and report — or with --download fetch — each model's full official doc set. Use this command to assemble doc sets for an entire rack at once. Do NOT use it for a single model; use 'literature get' instead.",
		Example: "  extron-pp-cli literature rack --bom ./rack.csv --json\n  extron-pp-cli literature rack --bom ./rack.csv --download --dir ./docs",
		Annotations: map[string]string{
			"pp:happy-args": "--bom=/dev/null",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "assemble rack docs")
			}
			if bomPath == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--bom is required (path to the rack bill of materials)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = resolveCatalogDB(flags, dbPath)
			db, ok := openCatalogReadOnly(cmd, dbPath)
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), make([]rackRow, 0), flags)
			}
			defer db.Close()
			hintIfCatalogIncomplete(cmd, db)

			models, err := readBOM(bomPath)
			if err != nil {
				return err
			}
			docs, err := loadCatalogDocs(ctx, db)
			if err != nil {
				return err
			}
			rows := make([]rackRow, 0, len(models))
			for _, m := range models {
				matched := resolveDocs(docs, m, perModel)
				row := rackRow{Model: m, DocCount: len(matched)}
				if len(matched) == 0 {
					row.Missing = true
				} else {
					row.Docs = matched
				}
				rows = append(rows, row)
			}
			if download {
				if err := downloadRackDocs(ctx, rows, dir); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No models in the BOM.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"model":     r.Model,
					"doc_count": r.DocCount,
					"missing":   r.Missing,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&bomPath, "bom", "", "path to the rack BOM file (required)")
	cmd.Flags().StringVar(&dir, "dir", "", "download directory (used with --download)")
	cmd.Flags().BoolVar(&download, "download", false, "download each model's doc set into --dir")
	cmd.Flags().IntVar(&perModel, "per-model", 5, "maximum docs to resolve per model")
	return cmd
}

// readBOM parses a BOM file: one model per line, or CSV with the model in
// the first column. Blank lines and header lines are skipped.
func readBOM(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening BOM: %w", err)
	}
	defer f.Close()
	br := bufio.NewReader(f)
	models := make([]string, 0, 16)
	lineNo := 0
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			lineNo++
			model := parseBOMModel(line)
			if model != "" {
				models = append(models, model)
			}
		}
		if err != nil {
			break
		}
	}
	if lineNo > 0 && len(models) == 0 {
		return nil, fmt.Errorf("no model numbers found in BOM %s", path)
	}
	return models, nil
}

func parseBOMModel(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	// CSV: the model is the first column; header detection applies to the
	// first cell so "Model Number,Description,Qty" is skipped as a header.
	first := line
	if strings.Contains(line, ",") {
		r := csv.NewReader(strings.NewReader(line))
		r.FieldsPerRecord = -1
		if rec, err := r.Read(); err == nil && len(rec) > 0 {
			first = strings.TrimSpace(rec[0])
		}
	}
	lower := strings.ToLower(first)
	if lower == "model" || lower == "model number" || lower == "model no" || lower == "qty" {
		return ""
	}
	return first
}

func downloadRackDocs(ctx context.Context, rows []rackRow, dir string) error {
	if dir == "" {
		return fmt.Errorf("--dir is required with --download")
	}
	client := extron.New()
	for _, r := range rows {
		if len(r.Docs) == 0 {
			continue
		}
		for _, d := range r.Docs {
			rel := downloadDest(dir, d)
			full, err := resolveLedgerPath(dir, rel)
			if err != nil {
				return fmt.Errorf("downloading %s: %w", d.Title, err)
			}
			n, err := client.Download(ctx, client.AbsoluteURL(d.URL), full)
			if err != nil {
				return fmt.Errorf("downloading %s: %w", d.Title, err)
			}
			// Record each success immediately so a later batch failure does
			// not leave earlier downloads outside revision/integrity tracking.
			if err := upsertLedgerRecords(dir, []downloadRecord{newDownloadRecord(d, rel, n)}); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "downloaded: %s -> %s\n", d.Title, rel)
		}
	}
	return nil
}
