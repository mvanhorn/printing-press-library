package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newListeCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "liste [comune]",
		Short: "Voti per lista elettorale collegata a ogni candidato sindaco.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli liste Agrigento
  elezioni-sicilia-pp-cli liste Palermo --json
  elezioni-sicilia-pp-cli liste Catania --anno 2024 --json --select candidati.liste`, "\n"),
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
				fmt.Fprintf(cmd.ErrOrStderr(), "would fetch: ReportCandidatiListe for %s\n", args[0])
				return nil
			}

			comune, err := scraper.ResolveComune(args[0], provincia, anno)
			if err != nil {
				return fmt.Errorf("liste: %w", err)
			}

			result, srcURL, err := scraper.FetchListe(comune, anno)
			if err != nil {
				return fmt.Errorf("liste: %w", err)
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

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Elezioni Comunali %d — %s (%s) — %s\n\n",
				anno, result.Comune, result.Provincia, result.Stato)
			for _, cand := range result.Candidati {
				fmt.Fprintf(cmd.OutOrStdout(), "Candidato %s: %s (voti: %s)\n", cand.Numero, cand.Nome, cand.Voti)
				listeRows := make([][]string, len(cand.Liste))
				for i, l := range cand.Liste {
					listeRows[i] = []string{l.Numero, l.Nome, l.Candidati, l.Voti, l.Percento}
				}
				_ = flags.printTable(cmd, []string{"N°", "LISTA", "CAND.", "VOTI", "%"}, listeRows)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}

// newListeFromSpec wraps newListeCmd for backward compat with generated promoted cmd.
func newListeFromSpec(flags *rootFlags) *cobra.Command {
	return newListeCmd(flags)
}

func formatLista(l scraper.Lista) string {
	return fmt.Sprintf("[%s] %s — voti: %s (%s%%)", l.Numero, l.Nome, l.Voti, strings.TrimSuffix(l.Percento, "%"))
}
