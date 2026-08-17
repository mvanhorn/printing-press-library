// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: coverage — per-model doc-type availability matrix for a
// category, flagging which models are missing manuals, spec sheets, etc.
// pp:data-source local
// Local-store command: pure local products ⋈ documents join over the corpus.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "coverage [category]",
		Short: "Per-model doc-type availability matrix for a category, flagging which models are missing manuals, spec sheets, or white papers.",
		Long: strings.Trim(`
Show which document types exist for every model in a category and which are
missing. The matrix is a local join of the harvested products and documents
tables — no AVer page offers this view.

Use this command to see which doc types exist per model in a category.
Do NOT use it to compare spec values; use 'compare' instead.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli coverage conference-camera
  averusa-pp-cli coverage --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<category>=conference-camera",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for coverage; it joins the local corpus after `harvest`"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("coverage takes at most one category, e.g. `coverage conference-camera`"))
			}
			category := ""
			if len(args) == 1 {
				category = args[0]
			}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus for coverage"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.CoverageAVERUSA(category)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return notFoundErr(fmt.Errorf("no products%sfound in the corpus; run `averusa-pp-cli harvest --only products` first", map[bool]string{true: " for category " + category + " ", false: " "}[category != ""]))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			// Compact text matrix: one line per model, "type:present" markers.
			w := cmd.OutOrStdout()
			for _, r := range rows {
				var present []string
				for _, t := range r.Types {
					present = append(present, t)
				}
				fmt.Fprintf(w, "%-14s %s\n", r.Model, strings.Join(present, ", "))
				if len(r.Missing) > 0 {
					fmt.Fprintf(w, "%-14s missing: %s\n", "", strings.Join(r.Missing, ", "))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	return cmd
}
