// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

type fascicoloVoce struct {
	Citazione     string `json:"citazione"`
	Etichetta     string `json:"etichetta"`
	URN           string `json:"urn"`
	URL           string `json:"url"`
	Titolo        string `json:"titolo,omitempty"`
	Intestazione  string `json:"intestazione,omitempty"`
	InizioVigenza string `json:"inizio_vigenza,omitempty"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	Abrogato      bool   `json:"abrogato"`
	Testo         string `json:"testo,omitempty"`
	Errore        string `json:"errore,omitempty"`
}

type fascicoloView struct {
	Data     string          `json:"data"`
	Voci     []fascicoloVoce `json:"voci"`
	Mancanti int             `json:"mancanti"`
	Markdown string          `json:"markdown,omitempty"`
	File     string          `json:"file,omitempty"`
}

func newNovelFascicoloCmd(flags *rootFlags) *cobra.Command {
	var al, file, output string
	cmd := &cobra.Command{
		Use:   "fascicolo [citazioni...]",
		Short: "Assemble several articles from different laws, all pinned to one reference date, into one Markdown document.",
		Long: strings.Trim(`
Build one Markdown document with the text of several norms, every one read
as in force on the same reference date (--al, default today): YAML front
matter with the date, then one section per norm with heading, text, vigenza
window and permalink. Citations come from arguments or from a text file
(--file) whose citations are extracted automatically.

Use this command to assemble several articles from different acts into one
document pinned to one date. Do NOT use it to read or export a whole single
act; use 'atto' instead. Do NOT use it to read one article; use 'testo'
instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli fascicolo "art. 2043 c.c." "art. 3 L. 241/1990" --al 2021-05-10
  normattiva-pp-cli fascicolo --file atto-di-citazione.txt --al 2019-03-01 --output norme.md`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "output", "pp:data-source": "auto",
			"pp:happy-args": "citazione=art. 2043 c.c.;--al=2021-05-10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fascicolo")
			}
			date, err := norma.ParseDate(al, timeNow())
			if err != nil {
				return usageErr(err)
			}
			var cits []norma.Citation
			if file != "" {
				data, rerr := readLimited(file)
				if rerr != nil {
					return usageErr(fmt.Errorf("reading %s: %w", file, rerr))
				}
				for _, c := range norma.ExtractCitations(string(data)) {
					if c.Articolo != "" {
						cits = append(cits, c)
					}
				}
			}
			for _, a := range args {
				c, perr := norma.ParseCitation(a, "")
				if perr != nil {
					return usageErr(perr)
				}
				if c.Articolo == "" {
					return usageErr(fmt.Errorf("%q cites a whole act; fascicolo collects articles (add \"art. N\")", a))
				}
				cits = append(cits, c)
			}
			if len(cits) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("no article citations given (arguments or --file)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := fascicoloView{Data: date, Voci: make([]fascicoloVoce, 0, len(cits))}
			seen := map[string]bool{}
			for _, cit := range cits {
				u := cit.URN().WithVigenza(date)
				if seen[u.String()] {
					continue
				}
				seen[u.String()] = true
				voce := fascicoloVoce{Citazione: cit.Raw, Etichetta: cit.Label(), URN: u.String(), URL: u.WebURL()}
				v, ferr := fetchArticle(ctx, c, flags, u)
				if ferr != nil {
					voce.Errore = ferr.Error()
					view.Mancanti++
				} else {
					voce.Titolo, voce.Intestazione, voce.InizioVigenza, voce.FineVigenza, voce.Abrogato = v.Titolo, v.Rubrica, v.InizioVigenza, v.FineVigenza, v.Abrogato
					voce.Testo = norma.RenderArticle(v.HTML, norma.RenderOptions{Markdown: true, Links: true})
				}
				view.Voci = append(view.Voci, voce)
			}
			if view.Mancanti > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d norms could not be read; they are listed with their error\n", view.Mancanti, len(view.Voci))
			}
			md := renderFascicolo(view)
			if output != "" {
				if err := writeFileAtomic(output, []byte(md)); err != nil {
					return fmt.Errorf("writing %s: %w", output, err)
				}
				view.File = output
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if output == "" {
					view.Markdown = md
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if output != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%d norms, reference date %s)\n", output, len(view.Voci)-view.Mancanti, date)
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), cliutil.EscapeTerminalStream(md))
			return nil
		},
	}
	cmd.Flags().StringVar(&al, "al", "", "Reference date for every norm (YYYY-MM-DD; default today)")
	cmd.Flags().StringVar(&file, "file", "", "Extract the article citations from this text file")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write the Markdown to this file instead of stdout")
	return cmd
}

func renderFascicolo(view fascicoloView) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "data_riferimento: %s\nfonte: Normattiva (https://www.normattiva.it)\nnorme: %d\n", view.Data, len(view.Voci))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# Fascicolo normativo al %s\n\n", view.Data)
	for _, v := range view.Voci {
		fmt.Fprintf(&b, "## %s\n\n", v.Etichetta)
		if v.Errore != "" {
			fmt.Fprintf(&b, "> Non disponibile: %s\n\n", v.Errore)
			continue
		}
		meta := v.Titolo
		if v.InizioVigenza != "" {
			if v.FineVigenza == "" {
				meta += fmt.Sprintf(" — testo vigente dal %s (tuttora in vigore)", v.InizioVigenza)
			} else {
				meta += fmt.Sprintf(" — testo vigente dal %s al %s", v.InizioVigenza, v.FineVigenza)
			}
		}
		if v.Abrogato {
			meta += " — **abrogato**"
		}
		fmt.Fprintf(&b, "_%s_ · [Normattiva](%s)\n\n", meta, v.URL)
		// Demote the article's own "## Art. N" heading under the section heading.
		testo := v.Testo
		if strings.HasPrefix(testo, "## ") {
			testo = "#" + testo
		}
		b.WriteString(strings.ReplaceAll(testo, "\n## ", "\n### "))
		b.WriteString("\n\n")
	}
	return b.String()
}
