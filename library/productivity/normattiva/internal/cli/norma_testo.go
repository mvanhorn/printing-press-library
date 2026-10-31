// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

func newTestoCmd(flags *rootFlags) *cobra.Command {
	var articolo, al, formato, codice, dataGU string
	var originale bool
	cmd := &cobra.Command{
		Use:   "testo [atto]",
		Short: "Read an article (or an act) as clean text, at today's date or any past date",
		Long: strings.Trim(`
Read the authoritative Normattiva text of an article.

The act can be a URN:NIR, a well-known name ("codice civile", "costituzione",
"gdpr" — see 'fonti'), or a citation as lawyers write it ("d.lgs. 196/2003",
"art. 3 L. 241/1990", "2043 c.c."). Use --al for the version in force on a
date and --originale for the text as first published. Acts that have no
verified URN form can be read from their Gazzetta coordinates with --codice
and --data-gu (current text only).`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli testo "codice civile" --articolo 2043
  normattiva-pp-cli testo "d.lgs. 196/2003" --articolo 13 --al 2010-01-01
  normattiva-pp-cli testo "art. 3 L. 241/1990" --formato markdown
  normattiva-pp-cli testo --codice 090G0294 --data-gu 1990-08-18 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "atto=codice civile;--articolo=2043"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "testo")
			}
			if err := validateFormat(formato); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var v articleView
			switch {
			case codice != "" || dataGU != "":
				if codice == "" || dataGU == "" {
					return usageErr(errors.New("--codice and --data-gu must be used together"))
				}
				if al != "" || originale {
					return usageErr(errors.New("Gazzetta coordinates always return today's text; use a URN with --al or --originale for other versions"))
				}
				d, derr := norma.ParseDate(dataGU, timeNow())
				if derr != nil {
					return usageErr(derr)
				}
				n := 0
				if articolo != "" {
					var aerr error
					if n, aerr = strconv.Atoi(strings.TrimSpace(articolo)); aerr != nil || n < 1 {
						return usageErr(fmt.Errorf("with --codice, --articolo must be a plain article number (got %q); use a URN for articles like 21-bis", articolo))
					}
				}
				v, err = fetchByGazzetta(ctx, c, codice, d, n)
			case len(args) == 0:
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required: a URN, a name like \"codice civile\", or a citation like \"d.lgs. 196/2003\""))
			default:
				u, _, rerr := resolveReference(strings.Join(args, " "), articolo)
				if rerr != nil {
					return usageErr(rerr)
				}
				u, rerr = applyVersion(u, al, originale)
				if rerr != nil {
					return rerr
				}
				v, err = fetchArticle(ctx, c, flags, u)
			}
			if err != nil {
				return articleErr(cmd, err, flags)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if formato == "markdown" || formato == "md" {
					v.Testo = articleBody(v, formato)
				}
				if formato != "html" {
					v.HTML = ""
				}
				return printJSONFiltered(cmd.OutOrStdout(), v, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", cliutil.ScrubTerminal(v.Titolo))
			if v.Sottotitolo != "" {
				fmt.Fprintf(w, "%s\n", cliutil.ScrubTerminal(v.Sottotitolo))
			}
			fmt.Fprintf(w, "%s · %s\n\n", vigenzaLabel(v), v.URL)
			fmt.Fprintln(w, cliutil.EscapeTerminalStream(articleBody(v, formato)))
			return nil
		},
	}
	cmd.Flags().StringVar(&articolo, "articolo", "", "Article number, e.g. 2043, 21-octies, 3-bis")
	cmd.Flags().StringVar(&al, "al", "", "Read the version in force on this date (YYYY-MM-DD, DD/MM/YYYY or 'oggi')")
	cmd.Flags().BoolVar(&originale, "originale", false, "Read the text as originally published, before any amendment")
	cmd.Flags().StringVar(&formato, "formato", "text", "Output body format: text, markdown or html")
	cmd.Flags().StringVar(&codice, "codice", "", "Gazzetta editorial code (codice redazionale), e.g. 090G0294; use with --data-gu")
	cmd.Flags().StringVar(&dataGU, "data-gu", "", "Gazzetta Ufficiale publication date (YYYY-MM-DD); use with --codice")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTestoCmd(flags))
	})
}
