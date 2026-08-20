package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/store"
	"github.com/spf13/cobra"
)

func newNormalizeReportCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	var outputPath string
	var kind string
	var format string
	var importStore bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "normalize-report",
		Short: "Normalize downloaded Amazon Ads report files for analytics commands",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return usageErr(fmt.Errorf("--input is required"))
			}
			report, err := adsanalytics.NormalizeReport(inputPath, kind)
			if err != nil {
				return err
			}
			out := map[string]any{
				"id":          report.ID,
				"kind":        report.Kind,
				"source_path": report.SourcePath,
				"row_count":   report.RowCount,
			}
			if outputPath != "" {
				if err := writeNormalizedReport(outputPath, format, report.Rows); err != nil {
					return err
				}
				out["output"] = outputPath
				out["format"] = format
			} else {
				out["rows"] = report.Rows
			}
			if importStore {
				if dbPath == "" {
					dbPath = defaultDBPath("amazon-ads-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()
				meta, err := db.ImportNormalizedReport(cmd.Context(), report.ID, report.Kind, report.SourcePath, report.Rows)
				if err != nil {
					return err
				}
				out["store"] = meta
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "Path to a downloaded report file; .gz files are decompressed automatically")
	cmd.Flags().StringVar(&kind, "kind", "generic", "Report kind: search-terms, performance, keyword-performance, or generic")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path for normalized rows; default prints JSON to stdout")
	cmd.Flags().StringVar(&format, "format", "json", "Output format when --output is set: json or jsonl")
	cmd.Flags().BoolVar(&importStore, "store", false, "Import normalized rows into the local SQLite store")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for --store")
	return cmd
}

func writeNormalizedReport(path, format string, rows []json.RawMessage) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating normalized report %s: %w", path, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	case "jsonl":
		for _, row := range rows {
			if _, err := fmt.Fprintln(w, string(row)); err != nil {
				return fmt.Errorf("writing normalized report %s: %w", path, err)
			}
		}
		return nil
	default:
		return usageErr(fmt.Errorf("--format must be json or jsonl"))
	}
}
