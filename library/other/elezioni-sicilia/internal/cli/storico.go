package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

// StoricoAnno holds affluenza data for one election year.
type StoricoAnno struct {
	Anno        int    `json:"anno"`
	Elettori    string `json:"elettori,omitempty"`
	Votanti     string `json:"votanti,omitempty"`
	Percentuale string `json:"percentuale,omitempty"`
	Errore      string `json:"errore,omitempty"`
}

// StoricoComune holds historical affluenza data for a comune across all known years.
type StoricoComune struct {
	Comune    string        `json:"comune"`
	Provincia string        `json:"provincia"`
	Anni      []StoricoAnno `json:"anni"`
}

func newStoricoCmd(flags *rootFlags) *cobra.Command {
	var provincia string

	cmd := &cobra.Command{
		Use:   "storico [comune]",
		Short: "Confronta affluenza di un comune in tutti gli anni disponibili (2009-2026).",
		Long: `Fetcha le pagine di affluenza per tutti gli anni disponibili sul sito
e mostra un confronto della partecipazione elettorale nel tempo.`,
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli storico Agrigento
  elezioni-sicilia-pp-cli storico Messina --json
  elezioni-sicilia-pp-cli storico Palermo --json --select anni`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"comune":"Agrigento","provincia":"AG","anni":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: ReportTabellaAffluenza.html for all known years")
				return nil
			}

			query := args[0]
			// First resolve the comune in the current year to get the name
			refComune, err := scraper.ResolveComune(query, provincia, 2026)
			if err != nil {
				// Try other years
				for _, y := range scraper.KnownYears {
					refComune, err = scraper.ResolveComune(query, provincia, y)
					if err == nil {
						break
					}
				}
				if err != nil {
					return fmt.Errorf("storico: comune %q non trovato", query)
				}
			}

			result := StoricoComune{
				Comune:    refComune.Nome,
				Provincia: refComune.Provincia,
			}

			for _, anno := range scraper.KnownYears {
				records, _, err := scraper.FetchAffluenza(anno)
				if err != nil {
					result.Anni = append(result.Anni, StoricoAnno{Anno: anno, Errore: err.Error()})
					continue
				}

				// Find this comune in the affluenza data
				query_lower := strings.ToLower(refComune.Nome)
				found := false
				for _, r := range records {
					if strings.ToLower(r.Comune) == query_lower {
						// Get last rilevamento
						var lastVotanti, lastPerc string
						if len(r.Rilevamenti) > 0 {
							last := r.Rilevamenti[len(r.Rilevamenti)-1]
							lastVotanti = last.Votanti
							lastPerc = last.Percentuale
						}
						result.Anni = append(result.Anni, StoricoAnno{
							Anno:        anno,
							Elettori:    r.Elettori,
							Votanti:     lastVotanti,
							Percentuale: lastPerc,
						})
						found = true
						break
					}
				}
				if !found {
					result.Anni = append(result.Anni, StoricoAnno{Anno: anno, Errore: "comune non presente"})
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				data, _ := json.MarshalIndent(result, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Storico affluenza — %s (%s)\n\n", result.Comune, result.Provincia)
			rows := make([][]string, 0, len(result.Anni))
			for _, a := range result.Anni {
				if a.Errore != "" {
					rows = append(rows, []string{fmt.Sprintf("%d", a.Anno), "-", "-", a.Errore})
				} else {
					perc := "-"
					if a.Percentuale != "" {
						perc = a.Percentuale + "%"
					}
					rows = append(rows, []string{fmt.Sprintf("%d", a.Anno), a.Elettori, a.Votanti, perc})
				}
			}
			return flags.printTable(cmd, []string{"ANNO", "ELETTORI", "VOTANTI", "% AFFLUENZA"}, rows)
		},
	}

	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia del comune (aiuta la ricerca)")
	return cmd
}
