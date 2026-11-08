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

type diffSide struct {
	Richiesta     string `json:"richiesta"`
	URN           string `json:"urn"`
	InizioVigenza string `json:"inizio_vigenza,omitempty"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	Abrogato      bool   `json:"abrogato"`
	Hash          string `json:"hash"`
}

type diffView struct {
	Citazione  string         `json:"citazione"`
	Etichetta  string         `json:"etichetta"`
	Titolo     string         `json:"titolo"`
	Articolo   string         `json:"articolo"`
	Da         diffSide       `json:"da"`
	A          diffSide       `json:"a"`
	Identico   bool           `json:"identico"`
	Aggiunte   int            `json:"parole_aggiunte"`
	Rimosse    int            `json:"parole_rimosse"`
	Diff       string         `json:"diff"`
	Operazioni []norma.DiffOp `json:"operazioni,omitempty"`
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var articolo, dal, al string
	var originale, ops, colore bool
	cmd := &cobra.Command{
		Use:   "diff [atto]",
		Short: "See word by word what an article said on one date versus another, or versus its original text.",
		Long: strings.Trim(`
Compare two versions of one article: the text in force on --dal (or the
original text with --originale) against the text in force on --al (default
today). Deleted words are shown as [-...-] and inserted words as {+...+};
both vigenza windows are reported, and identical versions say so.

Use this command to see exactly what changed in an article between two dates,
or between the original and the current text. Do NOT use it to list every
version window of an article; use 'cronologia' instead. Do NOT use it to read
one version; use 'testo' instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli diff "d.lgs. 196/2003" --articolo 2-ter --dal 2019-01-01
  normattiva-pp-cli diff "art. 3 L. 241/1990" --originale
  normattiva-pp-cli diff "codice civile" --articolo 2051 --dal 2000-01-01 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto",
			"pp:happy-args": "atto=d.lgs. 196/2003;--articolo=2-ter;--dal=2019-01-01"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required, e.g. \"d.lgs. 196/2003\" --articolo 13"))
			}
			if dal == "" && !originale {
				return usageErr(errors.New("say what to compare against: --dal <date> or --originale"))
			}
			u, cit, err := resolveReference(strings.Join(args, " "), articolo)
			if err != nil {
				return usageErr(err)
			}
			if u.Articolo == "" {
				return usageErr(errors.New("diff works on one article: add --articolo or cite it (\"art. 13 d.lgs. 196/2003\")"))
			}
			u.Vigenza, u.Originale = "", false
			left, err := applyVersion(u, dal, originale)
			if err != nil {
				return err
			}
			if al == "" {
				al = "oggi"
			}
			right, err := applyVersion(u, al, false)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			a, err := fetchArticle(ctx, c, flags, left)
			if err != nil {
				return articleErr(cmd, err, flags)
			}
			b, err := fetchArticle(ctx, c, flags, right)
			if err != nil {
				return articleErr(cmd, err, flags)
			}
			leftLabel := dal
			if originale {
				leftLabel = "originale"
			}
			d := norma.WordDiff(a.Testo, b.Testo)
			view := diffView{
				Citazione: cit.Raw,
				Etichetta: cit.Label(),
				Titolo:    b.Titolo,
				Articolo:  norma.DisplayArticle(u.Articolo),
				Da:        diffSide{Richiesta: leftLabel, URN: a.URN, InizioVigenza: a.InizioVigenza, FineVigenza: a.FineVigenza, Abrogato: a.Abrogato, Hash: a.Hash},
				A:         diffSide{Richiesta: right.Vigenza, URN: b.URN, InizioVigenza: b.InizioVigenza, FineVigenza: b.FineVigenza, Abrogato: b.Abrogato, Hash: b.Hash},
				Identico:  a.Hash == b.Hash,
				Diff:      norma.RenderDiff(d, false),
			}
			view.Aggiunte, view.Rimosse = norma.DiffStats(d)
			if ops {
				view.Operazioni = d
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s — art. %s\n", cliutil.ScrubTerminal(view.Titolo), view.Articolo)
			fmt.Fprintf(w, "  da: %-12s %s\n", leftLabel, vigenzaLabel(a))
			fmt.Fprintf(w, "  a:  %-12s %s\n\n", right.Vigenza, vigenzaLabel(b))
			if view.Identico {
				fmt.Fprintln(w, "The two versions are identical: no change between these dates.")
				return nil
			}
			useColor := colore && isTerminal(w) && !flags.plain
			// Scrub each run before coloring so upstream text cannot inject escapes.
			for _, op := range d {
				op.Text = cliutil.EscapeTerminalStream(op.Text)
				fmt.Fprint(w, norma.RenderDiff([]norma.DiffOp{op}, useColor))
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "\n+%d / -%d words\n", view.Aggiunte, view.Rimosse)
			return nil
		},
	}
	cmd.Flags().StringVar(&articolo, "articolo", "", "Article number, e.g. 13, 21-octies")
	cmd.Flags().StringVar(&dal, "dal", "", "Left side: the version in force on this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&al, "al", "", "Right side: the version in force on this date (default today)")
	cmd.Flags().BoolVar(&originale, "originale", false, "Left side: the text as originally published")
	cmd.Flags().BoolVar(&ops, "operazioni", false, "Include the structured diff operations in JSON output")
	cmd.Flags().BoolVar(&colore, "colore", false, "Also print a colored diff when writing to a terminal")
	return cmd
}
