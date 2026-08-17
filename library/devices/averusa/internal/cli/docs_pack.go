// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: docs pack — batch-download every document for a model into
// one offline folder with stable <model>-<type> names. The portal's own
// filenames are unstructured ("AVer PTZ Link User Manual EN v1.02_2021.07.12.pdf"),
// so deterministic naming is the point.
// pp:data-source local
// Local-store command: groups the corpus documents by model, then downloads
// each attached file through the fileField servlet.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/averusa"
	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/cliutil"
)

func newNovelDocsPackCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "pack <model>",
		Short: "Batch-download every document for a model into one offline folder with stable <model>-<type> names",
		Long: strings.Trim(`
Assemble the whole offline document bag for one model into a folder, with
stable <model>-<type> names (e.g. cam570-user-manual.pdf, cam570-spec-sheet.pdf,
cam570-datasheet.pdf). Use --dry-run to preview the bag without downloading.

Use this command to batch-download every document for a model into one folder.
Do NOT use it to fetch a single named document; use 'docs download <doc-id>' instead.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli docs pack CAM570 --out ./job-570
  averusa-pp-cli docs pack CAM570 --out ./job-570 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<model>=cam570;--out=/tmp/averusa-pack-test",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for docs pack; it groups the local corpus after `harvest`"))
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("docs pack requires exactly one model, e.g. `docs pack CAM570`"))
			}
			model := normalizeModel(args[0])
			if flagOut == "" {
				flagOut = model + "-docs"
			}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if dryRunOK(flags) {
					if flags.asJSON {
						return flags.printJSON(cmd, map[string]any{
							"model":   model,
							"out_dir": flagOut,
							"dry_run": true,
							"note":    "no local corpus yet; run `averusa-pp-cli harvest` first",
						})
					}
					return writeDryRun(cmd.OutOrStdout(), flags, "docs pack")
				}
				return notFoundErr(fmt.Errorf("no corpus for model %s", model))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			docs, err := st.DocumentsForModel(model, 200)
			if err != nil {
				return err
			}
			prods, err := st.ListAVERUSAProducts("", 200)
			if err != nil {
				return err
			}
			type packItem struct {
				Source string `json:"source"`
				Name   string `json:"name"`
				URL    string `json:"url"`
				Size   int64  `json:"size,omitempty"`
			}
			var items []packItem
			seen := map[string]bool{}
			for _, d := range docs {
				if d.PDFURL == "" || d.EntityID == "" {
					continue
				}
				name := stableDocName(model, d.DocType, d.EntityID, seen)
				items = append(items, packItem{Source: "portal", Name: name, URL: d.PDFURL})
			}
			for _, p := range prods {
				if normalizeModel(p.Slug) != model || p.DatasheetURL == "" {
					continue
				}
				name := model + "-datasheet.pdf"
				if !seen[name] {
					seen[name] = true
					items = append(items, packItem{Source: "averusa.com", Name: name, URL: p.DatasheetURL})
				}
			}
			if len(items) == 0 {
				// --dry-run previews the (empty) bag and still exits 0; the
				// not-found path applies only to a real download.
				if dryRunOK(flags) {
					if flags.asJSON {
						return flags.printJSON(cmd, map[string]any{
							"model":   model,
							"out_dir": flagOut,
							"dry_run": true,
							"items":   []packItem{},
						})
					}
					fmt.Fprintf(cmd.OutOrStdout(), "would pack 0 documents for %s into %s\n", model, flagOut)
					return nil
				}
				return notFoundErr(fmt.Errorf("no downloadable documents for model %q in the corpus; run `averusa-pp-cli harvest` first", model))
			}

			// --dry-run previews the bag without downloading anything.
			if dryRunOK(flags) {
				if flags.asJSON {
					return flags.printJSON(cmd, struct {
						Model  string     `json:"model"`
						OutDir string     `json:"out_dir"`
						DryRun bool       `json:"dry_run"`
						Items  []packItem `json:"items"`
					}{model, flagOut, true, items})
				}
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "would pack %d document(s) for %s into %s:\n", len(items), model, flagOut)
				for _, it := range items {
					fmt.Fprintf(w, "  %s  (%s)\n", it.Name, it.Source)
				}
				return nil
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download %d documents for %s into %s\n", len(items), model, flagOut)
				return nil
			}

			// Download with the same polite, rate-limited client as harvest.
			c := averusa.New()
			downloaded := 0
			var failures []string
			if err := os.MkdirAll(flagOut, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", flagOut, err)
			}
			for i := range items {
				it := &items[i]
				data, err := c.Download(ctx, it.URL)
				if err != nil {
					failures = appendBounded(failures, fmt.Sprintf("%s: %v", it.Name, err))
					continue
				}
				dst := filepath.Join(flagOut, it.Name)
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", dst, err)
				}
				it.Size = int64(len(data))
				downloaded++
			}
			rep := struct {
				Model      string     `json:"model"`
				OutDir     string     `json:"out_dir"`
				Downloaded int        `json:"downloaded"`
				Total      int        `json:"total"`
				Complete   bool       `json:"complete"`
				Items      []packItem `json:"items"`
				Failures   []string   `json:"failures,omitempty"`
			}{model, flagOut, downloaded, len(items), len(failures) == 0, items, failures}
			if flags.asJSON {
				if err := flags.printJSON(cmd, rep); err != nil {
					return err
				}
			} else {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "packed %d/%d documents for %s into %s\n", downloaded, len(items), model, flagOut)
				for _, it := range items {
					fmt.Fprintf(w, "  %s\n", it.Name)
				}
				for _, f := range failures {
					fmt.Fprintf(w, "  FAILED %s\n", f)
				}
			}
			if len(failures) > 0 {
				// Partial failure must not look like success: automation that
				// consumes the pack needs a non-zero exit to notice the gap.
				return apiErr(fmt.Errorf("%d of %d document(s) failed to download; the pack in %s is incomplete",
					len(failures), len(items), flagOut))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().StringVar(&flagOut, "out", "", "output directory (default: <model>-docs in the current directory)")
	return cmd
}

// stableDocName builds a deterministic <model>-<type> filename, deduping with
// a -2/-3 suffix when the corpus holds several docs of the same type.
func stableDocName(model, docType, entityID string, seen map[string]bool) string {
	base := model
	if docType != "" {
		base = model + "-" + docType
	}
	name := base + ".pdf"
	if !seen[name] {
		seen[name] = true
		return name
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d.pdf", base, i)
		if !seen[cand] {
			seen[cand] = true
			return cand
		}
	}
}
