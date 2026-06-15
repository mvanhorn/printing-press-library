// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelCitationsCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "citations <ecli>",
		Short: "Extract the vindplaatsen (citations) for a decision",
		Long: `Fetch a decision's RDF metadata and extract the dcterms:hasVersion
vindplaatsen list (where the decision is cited in legal journals like RvdW,
NJ, AB). Emit as JSON (default for agents), BibTeX (ready for legal-brief
footnotes), CSV (for spreadsheets), or plain text.`,
		Example: `  rechtspraak-pp-cli citations ECLI:NL:HR:2024:1
  rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex
  rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ecli := args[0]
			if _, err := rechtspraak.ParseECLI(ecli); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			d, err := mustHTTP().Get(ctx, ecli, false)
			if err != nil {
				return err
			}
			vs := d.Vindplaatsen
			format := strings.ToLower(flagFormat)
			if format == "" {
				if flags.asJSON {
					format = "json"
				} else if flags.csv {
					format = "csv"
				} else {
					format = "json" // default to JSON for agent-friendliness
				}
			}
			switch format {
			case "json":
				return writeJSONOut(cmd.OutOrStdout(), map[string]any{
					"ecli":         ecli,
					"vindplaatsen": vs,
				})
			case "csv":
				w := csv.NewWriter(cmd.OutOrStdout())
				_ = w.Write([]string{"raw", "journal", "year", "number", "page", "annotator"})
				for _, v := range vs {
					_ = w.Write([]string{v.Raw, v.Journal, v.Year, v.Number, v.Page, v.Annotator})
				}
				w.Flush()
				return w.Error()
			case "bibtex":
				// Entries with no journal/year/number are unresolved source
				// references (e.g. "Rechtspraak.nl" — the source repository
				// itself — or a bare slug like "SR-Updates.nl 2024-0003"
				// without volume/page metadata). BibTeX `@article` requires
				// a journal at minimum, so emit those as `@misc` and reserve
				// `@article` for entries that actually carry journal info.
				// This keeps legitimate journal cites (RvdW, NJ, AB) clean
				// for legal-brief footnotes.
				for i, v := range vs {
					key := bibtexKey(ecli, i)
					entryType := "article"
					if v.Journal == "" && v.Year == "" && v.Number == "" {
						entryType = "misc"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "@%s{%s,\n", entryType, key)
					if v.Journal != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  journal = {%s},\n", v.Journal)
					}
					if v.Year != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  year    = {%s},\n", v.Year)
					}
					if v.Number != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  number  = {%s},\n", v.Number)
					}
					if v.Annotator != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  note    = {m.nt. %s},\n", v.Annotator)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  ecli    = {%s},\n", ecli)
					fmt.Fprintf(cmd.OutOrStdout(), "  raw     = {%s}\n", v.Raw)
					fmt.Fprintln(cmd.OutOrStdout(), "}")
					fmt.Fprintln(cmd.OutOrStdout())
				}
				return nil
			case "text", "plain":
				for _, v := range vs {
					fmt.Fprintln(cmd.OutOrStdout(), v.Raw)
				}
				return nil
			default:
				return fmt.Errorf("unknown --format %q (expected: json|csv|bibtex|text)", format)
			}
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "", "Output format: json (default) | csv | bibtex | text")
	return cmd
}

func bibtexKey(ecli string, idx int) string {
	s := strings.ReplaceAll(ecli, ":", "_")
	return fmt.Sprintf("%s_%d", s, idx+1)
}
