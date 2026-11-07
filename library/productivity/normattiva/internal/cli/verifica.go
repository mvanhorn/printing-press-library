// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

// Citation check outcomes.
const (
	esitoOK          = "ok"
	esitoAbrogato    = "abrogato"
	esitoNonInVigore = "non_in_vigore"
	esitoNonTrovato  = "non_trovato"
	esitoModificato  = "modificato_dopo"
	esitoErrore      = "errore"
)

type verificaRow struct {
	Citazione     string `json:"citazione"`
	Etichetta     string `json:"etichetta"`
	URN           string `json:"urn"`
	URL           string `json:"url"`
	Esito         string `json:"esito"`
	Valida        bool   `json:"valida"`
	Titolo        string `json:"titolo,omitempty"`
	Intestazione  string `json:"intestazione,omitempty"`
	InizioVigenza string `json:"inizio_vigenza,omitempty"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	Nota          string `json:"nota,omitempty"`
}

type verificaView struct {
	Data      string        `json:"data"`
	Citazioni int           `json:"citazioni"`
	Valide    int           `json:"valide"`
	Problemi  int           `json:"problemi"`
	Risultati []verificaRow `json:"risultati"`
	Errori    []verificaRow `json:"fetch_failures,omitempty"`
}

func newNovelVerificaCmd(flags *rootFlags) *cobra.Command {
	var al, file string
	var soloProblemi bool
	cmd := &cobra.Command{
		Use:   "verifica [citazioni...]",
		Short: "Check every Italian law citation in a draft or an AI answer: does it exist, and was it in force on a given date?",
		Long: strings.Trim(`
Find every citation of an Italian State act in a text (or the citations
given as arguments), resolve each to its URN:NIR and check it on Normattiva:
does the act and article exist, was it in force on --al (default today),
has it been repealed, and did it change after that date.

Esiti: ok, modificato_dopo (valid on --al but amended since), non_in_vigore,
abrogato, non_trovato. Exit code 1 when any citation is not valid
(abrogato, non_in_vigore, non_trovato), so scripts and agents can gate on it.

Use this command to check citations that already appear in a piece of text
(or a list) against Normattiva. Do NOT use it to build a citation or link for
one known norm; use 'cita' instead. Do NOT use it to only parse or compose a
URN offline; use 'urn' instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli verifica "art. 2043 c.c." "art. 3 L. 241/1990" --al 2024-01-01 --agent
  normattiva-pp-cli verifica --file bozza.txt
  cat risposta.txt | normattiva-pp-cli verifica --file - --solo-problemi --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,1",
			"pp:happy-args": "citazione=art. 2043 c.c.;--al=2024-01-01"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verifica")
			}
			date := ""
			if al != "" {
				d, err := norma.ParseDate(al, timeNow())
				if err != nil {
					return usageErr(err)
				}
				date = d
			}
			var cits []norma.Citation
			switch {
			case file != "":
				var data []byte
				var err error
				if file == "-" {
					data, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 10<<20))
				} else {
					data, err = readLimited(file)
				}
				if err != nil {
					return usageErr(fmt.Errorf("reading %s: %w", file, err))
				}
				cits = norma.ExtractCitations(string(data))
			case len(args) > 0:
				for _, a := range args {
					// "artt. 2043 e 2059 c.c." expands to one citation per article.
					if many := norma.ExtractCitations(a); len(many) > 1 {
						cits = append(cits, many...)
						continue
					}
					c, err := norma.ParseCitation(a, "")
					if err != nil {
						return usageErr(err)
					}
					cits = append(cits, c)
				}
			default:
				_ = cmd.Usage()
				return usageErr(errors.New("give citations as arguments or a text with --file (use --file - for stdin)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			today := timeNow().Format("2006-01-02")
			view := verificaView{Data: date, Risultati: make([]verificaRow, 0, len(cits))}
			if view.Data == "" {
				view.Data = today
			}
			seen := map[string]bool{}
			for _, cit := range cits {
				u := cit.URN()
				if date != "" {
					u = u.WithVigenza(date)
				}
				key := u.String()
				if seen[key] {
					continue
				}
				seen[key] = true
				row := verificaRow{Citazione: cit.Raw, Etichetta: cit.Label(), URN: key, URL: u.WebURL()}
				v, ferr := fetchArticle(ctx, c, flags, u)
				switch {
				case errors.Is(ferr, errNotFound):
					row.Esito = esitoNonTrovato
					row.Nota = "Normattiva has no such act/article"
					if date != "" {
						row.Nota += " on " + date
					}
				case ferr != nil:
					row.Esito = esitoErrore
					row.Nota = ferr.Error()
					view.Errori = append(view.Errori, row)
					continue
				default:
					row.Titolo, row.Intestazione, row.InizioVigenza, row.FineVigenza = v.Titolo, v.Rubrica, v.InizioVigenza, v.FineVigenza
					switch {
					case u.Articolo == "":
						// Act-level citation: existence is what can be checked.
						row.Esito, row.Valida = esitoOK, true
						row.Nota = "act found; cite an article to check its vigenza"
					case v.Abrogato:
						row.Esito = esitoAbrogato
						row.Nota = firstLine(v.Testo, 1)
					case !v.InVigore:
						row.Esito = esitoNonInVigore
					case date != "" && date != today && v.FineVigenza != "":
						row.Esito, row.Valida = esitoModificato, true
						row.Nota = "the text in force on " + date + " was replaced on " + dayAfter(v.FineVigenza)
					default:
						row.Esito, row.Valida = esitoOK, true
					}
				}
				view.Citazioni++
				if row.Valida {
					view.Valide++
				} else {
					view.Problemi++
				}
				if soloProblemi && row.Valida {
					continue
				}
				view.Risultati = append(view.Risultati, row)
			}
			if len(view.Errori) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d citations could not be checked (network or service errors); counts cover the remaining %d\n", len(view.Errori), len(seen), view.Citazioni)
			}
			var outErr error
			switch {
			case view.Problemi > 0:
				outErr = &cliError{code: 1, err: fmt.Errorf("%d of %d citations are not valid", view.Problemi, view.Citazioni)}
			case len(view.Errori) > 0:
				outErr = apiErr(fmt.Errorf("%d citations could not be checked; nothing proves they are valid", len(view.Errori)))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return outErr
			}
			w := cmd.OutOrStdout()
			if view.Citazioni == 0 && len(view.Errori) == 0 {
				fmt.Fprintln(w, "No citations of Italian State acts found.")
				return nil
			}
			for _, r := range view.Risultati {
				mark := "✓"
				if !r.Valida {
					mark = "✗"
				}
				fmt.Fprintf(w, "%s %-16s %s", mark, r.Esito, cliutil.ScrubTerminal(r.Etichetta))
				if r.Intestazione != "" {
					fmt.Fprintf(w, " %s", cliutil.ScrubTerminal(r.Intestazione))
				}
				fmt.Fprintln(w)
				if r.Nota != "" {
					fmt.Fprintf(w, "    %s\n", cliutil.ScrubTerminal(r.Nota))
				}
				fmt.Fprintf(w, "    %s\n", r.URL)
			}
			for _, r := range view.Errori {
				fmt.Fprintf(w, "? %-16s %s\n    %s\n", "non_verificata", cliutil.ScrubTerminal(r.Etichetta), cliutil.ScrubTerminal(r.Nota))
			}
			fmt.Fprintf(w, "\n%d citations checked on %s: %d valid, %d with problems\n", view.Citazioni, view.Data, view.Valide, view.Problemi)
			return outErr
		},
	}
	cmd.Flags().StringVar(&al, "al", "", "Check validity on this date (YYYY-MM-DD; default today), e.g. the date of the facts")
	cmd.Flags().StringVar(&file, "file", "", "Read the text to scan from this file ('-' for stdin)")
	cmd.Flags().BoolVar(&soloProblemi, "solo-problemi", false, "Only list citations that are not valid")
	return cmd
}

func firstLine(s string, skip int) string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	if len(out) > skip {
		return out[skip]
	}
	if len(out) > 0 {
		return out[len(out)-1]
	}
	return ""
}

func dayAfter(d string) string {
	t, err := parseDay(d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}
