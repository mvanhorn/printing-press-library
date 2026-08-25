// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type comparedProduct struct {
	Requested    string `json:"requested"`
	Model        string `json:"model,omitempty"`
	Family       string `json:"family,omitempty"`
	URL          string `json:"url,omitempty"`
	SpecPDFURL   string `json:"spec_pdf_url,omitempty"`
	HasSpecText  bool   `json:"has_spec_text"`
	Discontinued bool   `json:"discontinued"`
	Found        bool   `json:"found"`
}

type compareResult struct {
	Products   []comparedProduct `json:"products"`
	SameFamily bool              `json:"same_family"`
	Missing    []string          `json:"missing"`
	Note       string            `json:"note,omitempty"`
}

func newNovelProductCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "compare [model-a] [model-b]",
		Short: "Compare two Q-SYS products side by side on spec text, family, and compatibility.",
		Long: strings.Trim(`
Compare puts two products next to each other using the local corpus.

Because specifications are stored as extracted text rather than parsed fields
(PDF layouts vary too much between Q-SYS product lines to parse reliably), this
compares what is known and links both source PDFs rather than inventing a
field-by-field table. For the authoritative numbers, open the two spec sheets it
returns - a fabricated comparison table would be worse than no table.
`, "\n"),
		Example:     "  qsys-pp-cli product compare CX-Q CXD-Q --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two product models are required, e.g. CX-Q CXD-Q"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "product compare")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), compareResult{
						Products: make([]comparedProduct, 0),
						Missing:  make([]string, 0),
					}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			res := compareResult{
				Products: make([]comparedProduct, 0, 2),
				Missing:  make([]string, 0),
			}
			for _, want := range args[:2] {
				cp := comparedProduct{Requested: want}
				p, found, err := findProduct(ctx, st.DB(), want)
				if err != nil {
					return err
				}
				if found {
					cp.Found = true
					cp.Model, cp.Family, cp.URL = p.Model, p.Family, p.URL
					cp.SpecPDFURL = p.SpecPDFURL
					cp.HasSpecText = p.SpecText != ""
					cp.Discontinued = p.Discontinued
				} else {
					res.Missing = append(res.Missing, want)
				}
				res.Products = append(res.Products, cp)
			}
			if len(res.Products) == 2 && res.Products[0].Found && res.Products[1].Found {
				res.SameFamily = res.Products[0].Family == res.Products[1].Family
			}
			if len(res.Missing) > 0 {
				res.Note = "not found locally: " + strings.Join(res.Missing, ", ") +
					" - run `qsys-pp-cli harvest --only products` or try the series name"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-22s %-22s\n", "FIELD", first(res, "model"), second(res, "model"))
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-22s %-22s\n", "family", first(res, "family"), second(res, "family"))
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-22s %-22s\n", "discontinued", first(res, "disc"), second(res, "disc"))
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-22s %-22s\n", "spec text", first(res, "spec"), second(res, "spec"))
			for _, p := range res.Products {
				if p.SpecPDFURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s spec sheet: %s", p.Model, p.SpecPDFURL)
				}
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if res.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	return cmd
}

func fieldOf(p comparedProduct, field string) string {
	if !p.Found {
		return "(not found)"
	}
	switch field {
	case "model":
		return p.Model
	case "family":
		return p.Family
	case "disc":
		return boolWord(p.Discontinued)
	case "spec":
		return boolWord(p.HasSpecText)
	}
	return ""
}

func first(r compareResult, field string) string {
	if len(r.Products) < 1 {
		return ""
	}
	return trimTo(fieldOf(r.Products[0], field), 22)
}

func second(r compareResult, field string) string {
	if len(r.Products) < 2 {
		return ""
	}
	return trimTo(fieldOf(r.Products[1], field), 22)
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Self-registration: this command was originally wired from the generated
// product resource wrapper, which a cross-spec regen re-emits from
// research.json novel_features and would drop. Registering from the preserved
// file keeps `product compare` available even when it is not a headline novel
// feature.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		productCmd, _, err := root.Find([]string{"product"})
		if err == nil {
			addNovelCommandIfAbsent(productCmd, newNovelProductCompareCmd(flags))
		}
	})
}
