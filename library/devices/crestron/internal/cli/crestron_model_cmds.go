// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Model-friendly wrappers over the path-based generated endpoint commands.
// Crestron product URLs encode the full category chain and cannot be built from
// a model number, so these resolve the model first via the local mirror.

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"

	"github.com/spf13/cobra"
)

type productGetView struct {
	crestronparse.Product
	Assets []crestronparse.Asset `json:"assets,omitempty"`
	Source string                `json:"source,omitempty"`
	Note   string                `json:"note,omitempty"`
}

func newCrestronProductGetCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagRelated bool

	cmd := &cobra.Command{
		Use:   "get <model>",
		Short: "Look up a product by model number",
		Long: strings.Trim(`
Look up a Crestron product by model number.

Crestron product URLs encode the full category chain, so a model number alone
cannot be turned into a URL. This resolves the model through the local mirror,
which 'sync' builds. Use 'product page' instead when you already know the
catalog path.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli product get DM-NVX-360
  crestron-pp-cli product get DM-NVX-360 --related --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "product get")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a model number is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}

			r, err := resolveModel(ctx, c, st, args[0])
			if err != nil && !needsSync(err) {
				return err
			}
			if err != nil {
				view := productGetView{Note: err.Error()}
				fmt.Fprintln(cmd.ErrOrStderr(), "run: crestron-pp-cli sync --resources categories,products")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			view := productGetView{Source: r.Source}
			if r.URL != "" {
				p, _, ferr := fetchProductPage(ctx, c, r.URL)
				if ferr == nil {
					view.Product = p
				}
			}
			if view.Model == "" {
				view.Model = r.Model
				view.URL = r.URL
				view.DocumentID = r.DocumentID
				view.Note = "only partial detail is available; run 'crestron-pp-cli sync' to build the local catalog"
			}
			if flagRelated {
				docID := view.DocumentID
				if docID == "" {
					docID = r.DocumentID
				}
				if assets, _, aerr := assetsForModel(ctx, c, view.Model, docID); aerr == nil {
					view.Assets = assets
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", view.Model)
			if view.Description != "" {
				fmt.Fprintf(out, "  %s\n", view.Description)
			}
			if view.URL != "" {
				fmt.Fprintf(out, "  url: https://www.crestron.com%s\n", strings.TrimPrefix(view.URL, "https://www.crestron.com"))
			}
			if view.Discontinued {
				fmt.Fprintln(out, "  status: discontinued")
			}
			if view.Note != "" {
				fmt.Fprintln(out, "  "+view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().BoolVar(&flagRelated, "related", false, "Also list the product's documentation assets")
	return cmd
}

type specsView struct {
	Model    string                      `json:"model"`
	Sections []crestronparse.SpecSection `json:"sections"`
	Rows     int                         `json:"row_count"`
	Note     string                      `json:"note,omitempty"`
}

func newCrestronSpecsShowCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "show <model>",
		Short: "Show a product's full specification table",
		Long: strings.Trim(`
Print a product's specification table as structured sections and key/value rows.

Use 'specs compare' to align two models against each other instead.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli specs show DM-NVX-360 --agent
  crestron-pp-cli specs show DM-NVX-360 --agent --select sections.name,sections.rows.key
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "specs show")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a model number is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}
			r, err := resolveModel(ctx, c, st, args[0])
			view := specsView{Model: args[0], Sections: make([]crestronparse.SpecSection, 0)}
			if err != nil && !needsSync(err) {
				return err
			}
			if err != nil {
				view.Note = err.Error()
				fmt.Fprintln(cmd.ErrOrStderr(), "run: crestron-pp-cli sync --resources categories,products")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			if r.URL == "" {
				view.Note = "no product page known for this model; run 'crestron-pp-cli sync' to build the local catalog"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			p, specs, err := fetchProductPage(ctx, c, r.URL)
			if err != nil {
				return fmt.Errorf("fetching specs for %s: %w", args[0], err)
			}
			if p.Model != "" {
				view.Model = p.Model
			}
			view.Sections = specs
			for _, s := range specs {
				view.Rows += len(s.Rows)
			}
			if view.Rows == 0 {
				view.Note = "this product page has no specification table"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Rows == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, s := range view.Sections {
				fmt.Fprintf(w, "\n%s\t\n", strings.ToUpper(s.Name))
				for _, row := range s.Rows {
					fmt.Fprintf(w, "  %s\t%s\n", truncateText(row.Key, 32), truncateText(row.Value, 70))
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if productCmd, _, err := root.Find([]string{"product"}); err == nil && productCmd != root {
			addNovelCommandIfAbsent(productCmd, newCrestronProductGetCmd(flags))
		}
		// specs show is wired directly in specs.go's parent constructor, which
		// runs before this hook.
	})
}
