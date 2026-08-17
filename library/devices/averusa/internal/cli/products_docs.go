// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// products docs — every harvested document (and the datasheet) for one model.
// pp:data-source local
// Local-store command: joins the corpus products and documents tables.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newProductsDocsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "docs <model>",
		Short: "List every document harvested for a model, plus its datasheet",
		Long: strings.Trim(`
List the documents harvested for one model (manuals, spec sheets, quick
starts, ...) plus the model's datasheet link, straight from the local corpus.
Use `+"`harvest`"+` first.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli products docs CAM570
  averusa-pp-cli products docs CAM570 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<model>=CAM570",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "products docs")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for products docs; it reads the corpus after `harvest`"))
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("products docs requires a model, e.g. `products docs CAM570`"))
			}
			model := normalizeModel(args[0])

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
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
			prods, err := st.ListAVERUSAProducts("", 500)
			if err != nil {
				return err
			}
			rep := struct {
				Model     string `json:"model"`
				Datasheet string `json:"datasheet_url,omitempty"`
				Documents []struct {
					Title   string `json:"title"`
					DocType string `json:"doc_type"`
					URLName string `json:"url_name"`
					HasFile bool   `json:"has_file"`
				} `json:"documents"`
			}{Model: model}
			for _, p := range prods {
				if normalizeModel(p.Slug) == model {
					rep.Datasheet = p.DatasheetURL
					break
				}
			}
			for _, d := range docs {
				rep.Documents = append(rep.Documents, struct {
					Title   string `json:"title"`
					DocType string `json:"doc_type"`
					URLName string `json:"url_name"`
					HasFile bool   `json:"has_file"`
				}{d.Title, d.DocType, d.URLName, d.HasFile})
			}
			if len(rep.Documents) == 0 && rep.Datasheet == "" {
				return notFoundErr(fmt.Errorf("no documents for model %q in the corpus; run `averusa-pp-cli harvest` first", model))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rep)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", strings.ToUpper(model))
			if rep.Datasheet != "" {
				fmt.Fprintf(w, "  datasheet: %s\n", rep.Datasheet)
			}
			for _, d := range rep.Documents {
				mark := " "
				if d.HasFile {
					mark = "*"
				}
				fmt.Fprintf(w, "%s [%s] %s (%s)\n", mark, d.DocType, d.Title, d.URLName)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	return cmd
}
