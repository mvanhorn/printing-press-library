// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// docs search — type-filtered full-text search over the harvested document
// catalog. The user's first-class filters are doc types (user manual, spec
// sheet, white paper), which no generated command models.
// pp:data-source local
// Local-store command: FTS over the corpus documents table.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newDocsSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var docType string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the document catalog, optionally filtered by doc type",
		Long: strings.Trim(`
Full-text search over the harvested document catalog (article titles and
bodies). Filter with --type to the doc types that matter: user-manual,
spec-sheet, white-paper, quick-start, software, brochure, comparison-chart,
article. Run `+"`harvest`"+` first — search reads the local corpus only.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli docs search "CAM570" --type user-manual
  averusa-pp-cli docs search "white paper" --type white-paper --limit 10 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<query>=CAM570;--type=user-manual",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docs search")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for docs search; it searches the corpus after `harvest`"))
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("docs search requires a query, e.g. `docs search \"CAM570\" --type user-manual`"))
			}
			query := args[0]
			if limit <= 0 {
				limit = 25
			}
			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus to search"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			hits, err := st.SearchAVERUSADocuments(query, docType, limit)
			if err != nil {
				return err
			}
			if len(hits) == 0 {
				return notFoundErr(fmt.Errorf("no documents matched %q%s", query, map[bool]string{true: " (type " + docType + ")", false: ""}[docType != ""]))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, hits)
			}
			w := cmd.OutOrStdout()
			for _, h := range hits {
				var d struct {
					Title   string `json:"title"`
					DocType string `json:"doc_type"`
					Model   string `json:"model"`
					URLName string `json:"url_name"`
					HasFile bool   `json:"has_file"`
				}
				_ = json.Unmarshal(h, &d)
				fileMark := " "
				if d.HasFile {
					fileMark = "*"
				}
				fmt.Fprintf(w, "%s %-12s %-12s %s\n", fileMark, d.DocType, d.Model, d.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().StringVar(&docType, "type", "", "doc type filter: user-manual, spec-sheet, white-paper, quick-start, software, brochure, comparison-chart, article")
	cmd.Flags().IntVar(&limit, "limit", 0, "max results (default 25)")
	return cmd
}
