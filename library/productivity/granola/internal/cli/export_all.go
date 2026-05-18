// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newExportAllCmd(flags *rootFlags) *cobra.Command {
	var last, since, until, outDir string
	var skipExisting bool
	var concurrency, limit int
	cmd := &cobra.Command{
		Use:   "export-all",
		Short: "Export many meetings to a directory of combined-markdown files",
		Long: `For each meeting matching the window, writes a combined-three-stream
markdown file (same shape as 'export'). Emits ndjson per meeting:
{id, status: exported|skipped|error, path, error}.`,
		Example: `  # Export the last 7 days of meetings
  granola-pp-cli export-all --out ~/Documents/meeting-transcripts --last 7d

  # Export a specific window, skipping anything already on disk
  granola-pp-cli export-all --out ~/Documents/meeting-transcripts \
    --since 2026-04-01 --until 2026-05-01 --skip-existing

  # Bulk export with concurrency
  granola-pp-cli export-all --out ./out --last 30d --concurrency 4`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return ioErr(err)
			}
			from, to, err := parseTimeWindow(last, since, until)
			if err != nil {
				return usageErr(err)
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			ids := selectDocsInWindow(c, from, to, limit)
			_ = concurrency // serial for now; bursting Granola has a rate-limit cost
			w := cmd.OutOrStdout()
			for _, id := range ids {
				out := filepath.Join(outDir, "export_"+id+".md")
				if skipExisting {
					if _, err := os.Stat(out); err == nil {
						_ = emitNDJSONLine(w, map[string]any{"id": id, "status": "skipped", "path": out})
						continue
					}
				}
				a, err := buildArtifacts(id, flags.dataSource != "local", "")
				if err != nil {
					_ = emitNDJSONLine(w, map[string]any{"id": id, "status": "error", "error": err.Error()})
					continue
				}
				body := composeFullMarkdown(a)
				if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
					_ = emitNDJSONLine(w, map[string]any{"id": id, "status": "error", "error": err.Error()})
					continue
				}
				_ = emitNDJSONLine(w, map[string]any{"id": id, "status": "exported", "path": out, "bytes": len(body)})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&last, "last", "", "Time window (e.g. 30d)")
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVarP(&outDir, "out", "o", "", "Output directory")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip writes where the target file already exists")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "Parallel export workers (reserved; currently serial)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap exports (0 = unlimited)")
	return cmd
}

// selectDocsInWindow returns the doc ids whose created_at lies inside the
// window, sorted newest-first. limit=0 returns all matches.
func selectDocsInWindow(c *granola.Cache, from, to time.Time, limit int) []string {
	all := c.SortedDocumentIDs()
	out := make([]string, 0, len(all))
	for _, id := range all {
		d := c.Documents[id]
		t, err := granola.ParseISO(d.CreatedAt)
		if err != nil || t.IsZero() {
			continue
		}
		if !from.IsZero() && t.Before(from) {
			continue
		}
		if !to.IsZero() && t.After(to) {
			continue
		}
		out = append(out, id)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Ensure fmt import is referenced
var _ = fmt.Sprintf
