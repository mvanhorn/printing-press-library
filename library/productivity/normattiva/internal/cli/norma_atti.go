// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

type attoArticle struct {
	Articolo      string `json:"articolo"`
	URN           string `json:"urn"`
	Intestazione  string `json:"intestazione,omitempty"`
	InizioVigenza string `json:"inizio_vigenza,omitempty"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	Abrogato      bool   `json:"abrogato"`
	Testo         string `json:"testo,omitempty"`
}

type attoView struct {
	URN                string        `json:"urn"`
	URL                string        `json:"url"`
	Titolo             string        `json:"titolo"`
	Sottotitolo        string        `json:"sottotitolo,omitempty"`
	Articoli           []attoArticle `json:"articoli"`
	Troncato           bool          `json:"troncato"`
	Motivo             string        `json:"motivo_troncamento,omitempty"`
	ArticoloSuccessivo string        `json:"articolo_successivo,omitempty"`
	RichiesteFatte     int           `json:"richieste"`
}

func newAttoReadCmd(flags *rootFlags) *cobra.Command {
	var al, formato string
	var originale, indice, noSuffissi bool
	var da string
	var maxArticoli, maxChars int
	cmd := &cobra.Command{
		Use:   "atto [atto]",
		Short: "Read a whole act article by article, with an index-only mode and a size budget",
		Long: strings.Trim(`
Read a whole act by walking its articles in order (1, 2, 2-bis, 2-ter, 3, ...).
Normattiva has no table-of-contents endpoint, so each article is one request;
--max-articoli and --max-chars bound the work and the output, and the result
says where to resume (--da). Use --indice for headings only.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli atto "L. 241/1990" --indice --max-articoli 10
  normattiva-pp-cli atto "costituzione" --da 1 --max-articoli 12 --formato markdown`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "atto=costituzione;--max-articoli=2;--no-suffissi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "atto")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required, e.g. \"L. 241/1990\" or \"costituzione\""))
			}
			if err := validateFormat(formato); err != nil {
				return err
			}
			startBase, startSuf, derr := parseStartArticle(da)
			if derr != nil {
				return usageErr(derr)
			}
			if maxArticoli < 1 {
				return usageErr(errors.New("--max-articoli must be at least 1"))
			}
			u, _, err := resolveReference(strings.Join(args, " "), "")
			if err != nil {
				return usageErr(err)
			}
			u = u.Act()
			if parsed, perr := norma.ParseURN(strings.Join(args, " ")); perr == nil && parsed.Allegato != "" {
				u.Allegato = parsed.Allegato
			}
			if cliutil.IsDogfoodEnv() && maxArticoli > 2 {
				maxArticoli = 2
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := attoView{URN: u.String(), URL: u.WebURL(), Articoli: make([]attoArticle, 0)}
			chars := 0
			fetch := func(art string) (articleView, error) {
				target, verr := applyVersion(u.WithArticle(art), al, originale)
				if verr != nil {
					return articleView{}, verr
				}
				view.RichiesteFatte++
				return fetchArticle(ctx, c, flags, target)
			}
			add := func(v articleView, art string) bool {
				a := attoArticle{Articolo: norma.DisplayArticle(art), URN: v.URN, Intestazione: v.Rubrica, InizioVigenza: v.InizioVigenza, FineVigenza: v.FineVigenza, Abrogato: v.Abrogato}
				if !indice {
					a.Testo = articleBody(v, formato)
				}
				if maxChars > 0 && chars+len(a.Testo) > maxChars && len(view.Articoli) > 0 {
					view.Troncato, view.Motivo, view.ArticoloSuccessivo = true, "max-chars", norma.DisplayArticle(art)
					return false
				}
				chars += len(a.Testo)
				view.Articoli = append(view.Articoli, a)
				if view.Titolo == "" {
					view.Titolo, view.Sottotitolo = v.Titolo, v.Sottotitolo
				}
				if len(view.Articoli) >= maxArticoli {
					view.Troncato, view.Motivo = true, "max-articoli"
					view.ArticoloSuccessivo = norma.DisplayArticle(nextCandidate(art, noSuffissi))
					return false
				}
				return true
			}
		walk:
			for n := startBase; ; n++ {
				base := strconv.Itoa(n)
				first := n == startBase
				if !(first && startSuf >= 0) {
					v, ferr := fetch(base)
					if ferr != nil {
						if errors.Is(ferr, errNotFound) {
							if first && len(view.Articoli) == 0 {
								return articleErr(cmd, ferr, flags)
							}
							break
						}
						return articleErr(cmd, ferr, flags)
					}
					if !add(v, base) {
						break
					}
				}
				if noSuffissi {
					continue
				}
				from := 0
				if first && startSuf >= 0 {
					from = startSuf
				}
				for _, suf := range norma.ArticleSuffixes[from:] {
					art := base + suf
					sv, serr := fetch(art)
					if serr != nil {
						if errors.Is(serr, errNotFound) {
							break
						}
						return articleErr(cmd, serr, flags)
					}
					if !add(sv, art) {
						break walk
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n%s\n%s\n\n", cliutil.ScrubTerminal(view.Titolo), cliutil.ScrubTerminal(view.Sottotitolo), view.URL)
			for _, a := range view.Articoli {
				state := ""
				if a.Abrogato {
					state = " [abrogato]"
				}
				if indice {
					fmt.Fprintf(w, "Art. %-10s %s%s\n", a.Articolo, cliutil.ScrubTerminal(a.Intestazione), state)
					continue
				}
				fmt.Fprintf(w, "%s\n\n", cliutil.EscapeTerminalStream(a.Testo))
			}
			if view.Troncato {
				fmt.Fprintf(w, "\n… stopped (%s) after %d articles; continue with --da %s\n", view.Motivo, len(view.Articoli), view.ArticoloSuccessivo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&al, "al", "", "Read every article as in force on this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&originale, "originale", false, "Read the text as originally published")
	cmd.Flags().StringVar(&formato, "formato", "text", "Article body format: text, markdown or html")
	cmd.Flags().BoolVar(&indice, "indice", false, "Only list article numbers and headings, without text")
	cmd.Flags().BoolVar(&noSuffissi, "no-suffissi", false, "Skip probing inserted articles (bis, ter, ...) to save requests")
	cmd.Flags().StringVar(&da, "da", "1", "Start from this article, e.g. 12 or 2-bis (the value printed when output is truncated)")
	cmd.Flags().IntVar(&maxArticoli, "max-articoli", 30, "Stop after this many articles (one request each)")
	cmd.Flags().IntVar(&maxChars, "max-chars", 80000, "Stop when the collected text exceeds this many characters (0 = no limit)")
	return cmd
}

// parseStartArticle splits "12" or "2-bis" into a base number and a suffix
// index (-1 for the base article itself).
func parseStartArticle(da string) (int, int, error) {
	a := norma.NormalizeArticle(da)
	i := 0
	for i < len(a) && a[i] >= '0' && a[i] <= '9' {
		i++
	}
	n, err := strconv.Atoi(a[:i])
	if err != nil || n < 1 {
		return 0, 0, fmt.Errorf("--da must be an article like 12 or 2-bis (got %q)", da)
	}
	if i == len(a) {
		return n, -1, nil
	}
	for k, suf := range norma.ArticleSuffixes {
		if a[i:] == suf {
			return n, k, nil
		}
	}
	return 0, 0, fmt.Errorf("--da: unknown article suffix %q", a[i:])
}

// nextCandidate is the article to try after art in reading order.
func nextCandidate(art string, noSuffissi bool) string {
	n, k, err := parseStartArticle(art)
	if err != nil {
		return art
	}
	switch {
	case noSuffissi || k == len(norma.ArticleSuffixes)-1:
		return strconv.Itoa(n + 1)
	case k < 0:
		return strconv.Itoa(n) + norma.ArticleSuffixes[0]
	default:
		return strconv.Itoa(n) + norma.ArticleSuffixes[k+1]
	}
}

func newFontiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fonti [ricerca]",
		Short: "List the well-known acts you can call by name (codice civile, costituzione, gdpr, tuel, ...)",
		Example: strings.Trim(`
  normattiva-pp-cli fonti
  normattiva-pp-cli fonti codice
  normattiva-pp-cli fonti privacy --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fonti")
			}
			rows := norma.SearchFonti(strings.Join(args, " "))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No well-known act matches; use a citation like \"d.lgs. 196/2003\" or a URN instead.")
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, f := range rows {
				names := append(append([]string{}, f.Abbreviazioni...), f.Alias...)
				if len(names) > 4 {
					names = names[:4]
				}
				table = append(table, []string{f.Key, f.Nome, f.Estremi, strings.Join(names, ", ")})
			}
			return flags.printTable(cmd, []string{"KEY", "NOME", "ESTREMI", "ALIAS"}, table)
		},
	}
	return cmd
}

type urnView struct {
	URN       string `json:"urn"`
	URL       string `json:"url"`
	Tipo      string `json:"tipo"`
	Data      string `json:"data"`
	Numero    string `json:"numero,omitempty"`
	Allegato  string `json:"allegato,omitempty"`
	Articolo  string `json:"articolo,omitempty"`
	Vigenza   string `json:"vigenza,omitempty"`
	Originale bool   `json:"originale"`
	Fonte     string `json:"fonte,omitempty"`
	Etichetta string `json:"etichetta"`
}

func newURNCmd(flags *rootFlags) *cobra.Command {
	var articolo, al string
	var originale bool
	cmd := &cobra.Command{
		Use:   "urn [riferimento]",
		Short: "Build or parse a URN:NIR identifier offline from a citation, a name or a URN",
		Example: strings.Trim(`
  normattiva-pp-cli urn "d.lgs. 196/2003 art. 13"
  normattiva-pp-cli urn "codice civile" --articolo 2043 --al 2020-01-01
  normattiva-pp-cli urn "urn:nir:stato:legge:1990-08-07;241~art3" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "riferimento=d.lgs. 196/2003 art. 13"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "urn")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("a reference is required"))
			}
			u, cit, err := resolveReference(strings.Join(args, " "), articolo)
			if err != nil {
				return usageErr(err)
			}
			if u, err = applyVersion(u, al, originale); err != nil {
				return err
			}
			cit.Articolo = u.Articolo
			view := urnView{URN: u.String(), URL: u.WebURL(), Tipo: u.Tipo, Data: u.Data, Numero: u.Numero, Allegato: u.Allegato, Articolo: u.Articolo, Vigenza: u.Vigenza, Originale: u.Originale, Fonte: cit.Fonte, Etichetta: cit.Label()}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", view.URN, view.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&articolo, "articolo", "", "Attach this article number")
	cmd.Flags().StringVar(&al, "al", "", "Pin the version in force on this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&originale, "originale", false, "Pin the originally published version")
	return cmd
}

type citaView struct {
	Citazione  string `json:"citazione"`
	Markdown   string `json:"markdown"`
	URN        string `json:"urn"`
	URL        string `json:"url"`
	Titolo     string `json:"titolo,omitempty"`
	Verificato bool   `json:"verificato"`
}

func newCitaCmd(flags *rootFlags) *cobra.Command {
	var articolo, al string
	var offline bool
	cmd := &cobra.Command{
		Use:   "cita [riferimento]",
		Short: "Build a ready-to-paste citation with a normattiva.it permalink for one norm",
		Long: strings.Trim(`
Build a citation (plain and Markdown) with a permanent normattiva.it link.
By default the norm is looked up once to confirm it exists and to take its
official title; --offline skips the lookup.

Use this command to build a citation for one known norm. Do NOT use it to
check citations already written in a text; use 'verifica' instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli cita "art. 2043 c.c."
  normattiva-pp-cli cita "L. 241/1990" --articolo 3 --al 2015-01-01 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "riferimento=art. 2043 c.c."},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cita")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("a reference is required, e.g. \"art. 2043 c.c.\""))
			}
			u, cit, err := resolveReference(strings.Join(args, " "), articolo)
			if err != nil {
				return usageErr(err)
			}
			if u, err = applyVersion(u, al, false); err != nil {
				return err
			}
			cit.Articolo = u.Articolo
			label := cit.Label()
			if u.Vigenza != "" {
				label += fmt.Sprintf(" (testo vigente al %s)", u.Vigenza)
			}
			view := citaView{Citazione: label, URN: u.String(), URL: u.WebURL()}
			if !offline {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				v, ferr := fetchArticle(ctx, c, flags, u)
				if ferr != nil {
					return articleErr(cmd, ferr, flags)
				}
				view.Titolo, view.Verificato = v.Titolo, true
				if cit.Fonte == "" && v.Titolo != "" {
					view.Citazione = strings.TrimSpace(strings.Replace(label, cit.Label(), labelFromTitle(u, v.Titolo), 1))
				}
			}
			view.Markdown = fmt.Sprintf("[%s](%s)", view.Citazione, view.URL)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", cliutil.ScrubTerminal(view.Citazione), cliutil.ScrubTerminal(view.Markdown))
			return nil
		},
	}
	cmd.Flags().StringVar(&articolo, "articolo", "", "Article number")
	cmd.Flags().StringVar(&al, "al", "", "Cite the version in force on this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&offline, "offline", false, "Do not contact Normattiva; build the citation from the reference alone")
	return cmd
}

// labelFromTitle renders "art. 3 L. 7 agosto 1990, n. 241" from the official title.
func labelFromTitle(u norma.URN, titolo string) string {
	t := titolo
	for _, pair := range [][2]string{{"DECRETO LEGISLATIVO", "D.Lgs."}, {"DECRETO-LEGGE", "D.L."}, {"DECRETO DEL PRESIDENTE DELLA REPUBBLICA", "D.P.R."}, {"REGIO DECRETO", "R.D."}, {"LEGGE", "L."}} {
		if strings.HasPrefix(t, pair[0]+" ") {
			t = pair[1] + t[len(pair[0]):]
			break
		}
	}
	if u.Articolo == "" {
		return t
	}
	return fmt.Sprintf("art. %s %s", norma.DisplayArticle(u.Articolo), t)
}

type versione struct {
	N             int    `json:"n"`
	URN           string `json:"urn"`
	InizioVigenza string `json:"inizio_vigenza"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	Abrogato      bool   `json:"abrogato"`
	Hash          string `json:"hash"`
	Testo         string `json:"testo,omitempty"`
}

type cronologiaView struct {
	URN      string     `json:"urn"`
	Titolo   string     `json:"titolo"`
	Articolo string     `json:"articolo"`
	Versioni []versione `json:"versioni"`
	Completa bool       `json:"completa"`
	Nota     string     `json:"nota,omitempty"`
}

func newCronologiaCmd(flags *rootFlags) *cobra.Command {
	var articolo string
	var maxVersioni int
	var conTesto bool
	cmd := &cobra.Command{
		Use:   "cronologia [atto]",
		Short: "List every version of an article with its vigenza window, from the original text to today",
		Long: strings.Trim(`
Walk every version of an article: start from the original text, then ask for
the version in force the day after each window ends, until the open-ended
current version. One request per version; --max-versioni bounds the walk.

Use this command to list the version windows of an article. Do NOT use it to
see what changed between two dates; use 'diff' instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli cronologia "d.lgs. 196/2003" --articolo 13
  normattiva-pp-cli cronologia "L. 241/1990" --articolo 3 --testo --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "atto=d.lgs. 196/2003;--articolo=13;--max-versioni=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cronologia")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required"))
			}
			u, _, err := resolveReference(strings.Join(args, " "), articolo)
			if err != nil {
				return usageErr(err)
			}
			if u.Articolo == "" {
				return usageErr(errors.New("cronologia works on one article: add --articolo or cite it (\"art. 13 d.lgs. 196/2003\")"))
			}
			if cliutil.IsDogfoodEnv() && maxVersioni > 2 {
				maxVersioni = 2
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			versions, title, complete, err := walkVersions(cmd, c, flags, u, maxVersioni)
			if err != nil {
				return articleErr(cmd, err, flags)
			}
			view := cronologiaView{URN: u.Act().WithArticle(u.Articolo).String(), Titolo: title, Articolo: norma.DisplayArticle(u.Articolo), Versioni: make([]versione, 0, len(versions)), Completa: complete}
			for i, v := range versions {
				ver := versione{N: i + 1, URN: v.URN, InizioVigenza: v.InizioVigenza, FineVigenza: v.FineVigenza, Abrogato: v.Abrogato, Hash: v.Hash}
				if conTesto {
					ver.Testo = v.Testo
				}
				view.Versioni = append(view.Versioni, ver)
			}
			if !complete {
				view.Nota = fmt.Sprintf("stopped after %d versions; raise --max-versioni to continue", maxVersioni)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — art. %s\n\n", cliutil.ScrubTerminal(view.Titolo), view.Articolo)
			rows := make([][]string, 0, len(view.Versioni))
			for _, v := range view.Versioni {
				end := v.FineVigenza
				if end == "" {
					end = "in vigore"
				}
				note := ""
				if v.Abrogato {
					note = "abrogato"
				}
				rows = append(rows, []string{strconv.Itoa(v.N), v.InizioVigenza, end, v.Hash, note})
			}
			if err := flags.printTable(cmd, []string{"#", "DAL", "AL", "HASH", "NOTE"}, rows); err != nil {
				return err
			}
			if view.Nota != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Nota)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&articolo, "articolo", "", "Article number")
	cmd.Flags().IntVar(&maxVersioni, "max-versioni", 25, "Stop after this many versions (one request each)")
	cmd.Flags().BoolVar(&conTesto, "testo", false, "Include the text of every version")
	return cmd
}

// walkVersions follows contiguous vigenza windows from the original text.
func walkVersions(cmd *cobra.Command, c *client.Client, flags *rootFlags, u norma.URN, max int) ([]articleView, string, bool, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	out := make([]articleView, 0)
	cur := u.WithOriginale()
	title := ""
	seen := map[string]bool{}
	for len(out) < max {
		v, err := fetchArticle(ctx, c, flags, cur)
		if err != nil {
			if len(out) > 0 && errors.Is(err, errNotFound) {
				return out, title, true, nil
			}
			return out, title, false, err
		}
		if seen[v.InizioVigenza] {
			return out, title, true, nil
		}
		seen[v.InizioVigenza] = true
		title = v.Titolo
		out = append(out, v)
		if v.FineVigenza == "" {
			return out, title, true, nil
		}
		end, perr := time.Parse("2006-01-02", v.FineVigenza)
		if perr != nil {
			return out, title, false, perr
		}
		cur = u.WithVigenza(end.AddDate(0, 0, 1).Format("2006-01-02"))
	}
	return out, title, false, nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newAttoReadCmd(flags))
		addNovelCommandIfAbsent(root, newFontiCmd(flags))
		addNovelCommandIfAbsent(root, newURNCmd(flags))
		addNovelCommandIfAbsent(root, newCitaCmd(flags))
		addNovelCommandIfAbsent(root, newCronologiaCmd(flags))
	})
}
