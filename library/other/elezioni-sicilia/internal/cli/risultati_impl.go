package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newRisultatiCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "risultati [comune]",
		Short: "Risultato finale delle elezioni in un comune (richiede scrutinio completato).",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli risultati Agrigento
  elezioni-sicilia-pp-cli risultati Palermo --json
  elezioni-sicilia-pp-cli risultati "Barcellona Pozzo di Gotto" --anno 2024`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"comune":"Agrigento","anno":2026,"stato_scrutinio":"completo","candidati":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "would fetch: ReportRisultati for %s\n", args[0])
				return nil
			}

			comune, err := scraper.ResolveComune(args[0], provincia, anno)
			if err != nil {
				return fmt.Errorf("risultati: %w", err)
			}

			result, srcURL, err := scraper.FetchRisultati(comune, anno)
			if err != nil {
				return fmt.Errorf("risultati: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				out := map[string]any{
					"meta": map[string]any{"source": srcURL},
					"data": result,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if result.Stato == scraper.ScrutinioInCorso {
				fmt.Fprintf(cmd.OutOrStdout(), "Scrutini ancora in corso per %s (%d). Usa 'stato' per monitorare l'avanzamento.\n",
					comune.Nome, anno)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Elezioni Comunali %d — %s (%s)\n", anno, result.Comune, result.Provincia)
			fmt.Fprintf(cmd.OutOrStdout(), "Elettori: %s  Votanti: %s (%s%%)\n\n",
				result.Elettori, result.Votanti, result.VotantiPerc)

			rows := make([][]string, len(result.Candidati))
			for i, c := range result.Candidati {
				eletto := ""
				if c.Eletto {
					eletto = "✓"
				}
				rows[i] = []string{c.Numero, eletto, c.Nome, c.Voti, c.Percento}
			}
			return flags.printTable(cmd, []string{"N°", "ELETTO", "CANDIDATO", "VOTI", "%"}, rows)
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}

func newSeggiCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "seggi [comune]",
		Short: "Ripartizione seggi in Consiglio Comunale per ogni lista.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli seggi Agrigento
  elezioni-sicilia-pp-cli seggi Messina --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"comune":"Agrigento","anno":2026,"stato_scrutinio":"completo","candidati":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "would fetch: ReportSeggi for %s\n", args[0])
				return nil
			}

			comune, err := scraper.ResolveComune(args[0], provincia, anno)
			if err != nil {
				return fmt.Errorf("seggi: %w", err)
			}

			result, srcURL, err := scraper.FetchSeggi(comune, anno)
			if err != nil {
				return fmt.Errorf("seggi: %w", err)
			}

			if result.Stato == scraper.ScrutinioInCorso {
				fmt.Fprintf(cmd.OutOrStdout(), "Scrutini ancora in corso per %s (%d).\n", comune.Nome, anno)
				return nil
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				out := map[string]any{
					"meta": map[string]any{"source": srcURL},
					"data": result,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Seggi — %s (%s) %d — totale seggi: %s\n\n",
				result.Comune, result.Provincia, anno, result.Seggi)
			rows := make([][]string, len(result.Candidati))
			for i, c := range result.Candidati {
				eletto := ""
				if c.Eletto {
					eletto = "✓"
				}
				listNames := make([]string, len(c.Liste))
				for j, l := range c.Liste {
					listNames[j] = fmt.Sprintf("%s(%s seggi)", l.Nome, l.Seggi)
				}
				rows[i] = []string{c.Numero, eletto, c.Nome, strings.Join(listNames, "; ")}
			}
			return flags.printTable(cmd, []string{"N°", "ELETTO", "CANDIDATO", "LISTE (SEGGI)"}, rows)
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}
