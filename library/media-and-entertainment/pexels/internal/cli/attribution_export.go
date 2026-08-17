// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/store"
)

type attributionExportResult struct {
	Exported   int    `json:"exported"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
}

func newNovelAttributionExportCmd(flags *rootFlags) *cobra.Command {
	var (
		flagResources string
		flagCSV       bool
		flagOutput    string
		flagDB        string
	)

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Emit a SOURCES.md (or CSV) crediting every photographer in your local download ledger.",
		Example:     "--resources photos,videos --csv --output SOURCES.csv",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--resources=photos"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export attribution for your local download ledger to a SOURCES file")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("attribution export has no live data source; it reads the local download ledger"))
			}

			resources := mediaTypesFromResources(splitResources(flagResources, ""))

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("pexels-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local mirror; run `pexels-pp-cli download ...` first")
				return printJSONFiltered(cmd.OutOrStdout(), make([]attributionExportResult, 0), flags)
			}

			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()
			db := st.DB()
			if err := store.EnsurePexelsDownloads(db); err != nil {
				return fmt.Errorf("ensure ledger: %w", err)
			}

			rows, err := store.AllPexelsDownloads(db, resources)
			if err != nil {
				return fmt.Errorf("read ledger: %w", err)
			}

			format := "markdown"
			if flagCSV {
				format = "csv"
			}

			if flagCSV {
				if err := writeAttributionCSV(flagOutput, rows); err != nil {
					return err
				}
			} else {
				if err := writeAttributionMarkdown(flagOutput, rows); err != nil {
					return err
				}
			}

			out := attributionExportResult{Exported: len(rows), OutputPath: flagOutput, Format: format}
			stdout := cmd.OutOrStdout()
			if flags.asJSON || flags.agent || !isTerminal(stdout) {
				return printJSONFiltered(stdout, out, flags)
			}
			fmt.Fprintf(stdout, "Exported %d attributions (%s) to %s\n", out.Exported, out.Format, out.OutputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagResources, "resources", "photos,videos", "comma-separated media types to include")
	cmd.Flags().BoolVar(&flagCSV, "csv", false, "emit CSV instead of Markdown")
	cmd.Flags().StringVar(&flagOutput, "output", "SOURCES.md", "output file path")
	cmd.Flags().StringVar(&flagDB, "db", "", "download ledger DB path (default: standard data dir)")
	return cmd
}

// mediaTypesFromResources normalizes resource names ("photos"/"videos", the
// flag vocabulary) to the singular media_type values stored in the ledger
// ("photo"/"video"). Unknown values pass through unchanged.
func mediaTypesFromResources(resources []string) []string {
	out := make([]string, 0, len(resources))
	for _, r := range resources {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case "photos", "photo":
			out = append(out, "photo")
		case "videos", "video":
			out = append(out, "video")
		default:
			if r != "" {
				out = append(out, r)
			}
		}
	}
	return out
}

func attributionText(d store.PexelsDownload) string {
	kind := "Photo"
	if d.MediaType == "video" {
		kind = "Video"
	}
	return fmt.Sprintf("%s by %s on Pexels (%s)", kind, d.Photographer, d.PageURL)
}

func writeAttributionMarkdown(outPath string, rows []store.PexelsDownload) error {
	var b strings.Builder
	b.WriteString("# Sources\n\n")
	b.WriteString("Media sourced from [Pexels](https://www.pexels.com). Credits below.\n\n")
	for _, d := range rows {
		b.WriteString("- ")
		b.WriteString(attributionText(d))
		b.WriteString("\n")
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o600)
}

func writeAttributionCSV(outPath string, rows []store.PexelsDownload) error {
	// #nosec G304 -- outPath is the user-chosen export destination passed via
	// the --out flag; writing the attribution report there is the command's job.
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create CSV: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"media_id", "media_type", "photographer", "page_url", "attribution"}); err != nil {
		return err
	}
	for _, d := range rows {
		rec := []string{
			strconv.FormatInt(d.MediaID, 10),
			d.MediaType,
			d.Photographer,
			d.PageURL,
			attributionText(d),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
