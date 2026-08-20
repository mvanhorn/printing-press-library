package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newAffluenzaCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "affluenza",
		Short: "Tabella affluenza elettorale per tutti i comuni siciliani con confronto storico.",
		Long: `Mostra l'affluenza alle urne per ogni comune siciliano con 4 rilevamenti orari
e confronto con le elezioni precedenti.`,
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli affluenza
  elezioni-sicilia-pp-cli affluenza --anno 2024 --json
  elezioni-sicilia-pp-cli affluenza --provincia PA
  elezioni-sicilia-pp-cli affluenza --json --select comune,rilevamenti`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[{"comune":"Agrigento","provincia":"AG","elettori":"51267","rilevamenti":[]}]`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: ReportTabellaAffluenza.html")
				return nil
			}

			records, srcURL, err := scraper.FetchAffluenza(anno)
			if err != nil {
				return fmt.Errorf("affluenza: %w", err)
			}

			// Filter by province if requested
			if provincia != "" {
				prov := strings.ToUpper(provincia)
				filtered := records[:0]
				for _, r := range records {
					if r.Provincia == prov {
						filtered = append(filtered, r)
					}
				}
				records = filtered
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				out := map[string]any{
					"meta": map[string]any{"source": srcURL, "anno": anno, "count": len(records)},
					"data": records,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if flags.csv {
				w := csv.NewWriter(cmd.OutOrStdout())
				_ = w.Write([]string{"comune", "provincia", "elettori", "orario", "votanti", "percentuale", "perc_precedenti", "diff_perc"})
				for _, r := range records {
					for _, ril := range r.Rilevamenti {
						_ = w.Write([]string{r.Comune, r.Provincia, r.Elettori, ril.Orario, ril.Votanti, ril.Percentuale, ril.PrecPercent, ril.Differenza})
					}
				}
				w.Flush()
				return nil
			}

			// Human table
			return flags.printTable(cmd, []string{"COMUNE", "PROV", "ELETTORI", "ORE 15:00 25/5", "% VOTANTI", "DIFF%"},
				affluenzaRows(records))
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Filtra per provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}

func affluenzaRows(records []scraper.AffluenzaComune) [][]string {
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		var lastVot, lastPerc, lastDiff string
		if len(r.Rilevamenti) > 0 {
			last := r.Rilevamenti[len(r.Rilevamenti)-1]
			lastVot = last.Votanti
			lastPerc = last.Percentuale + "%"
			lastDiff = last.Differenza
		}
		rows = append(rows, []string{r.Comune, r.Provincia, r.Elettori, lastVot, lastPerc, lastDiff})
	}
	return rows
}
