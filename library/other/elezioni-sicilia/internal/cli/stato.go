package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

// StatoComune represents the scrutinio state for one comune.
type StatoComune struct {
	Comune    string                 `json:"comune"`
	Provincia string                 `json:"provincia"`
	Stato     scraper.ScrutinioState `json:"stato"`
	Dettaglio string                 `json:"dettaglio,omitempty"`
}

func newStatoCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "stato [comune]",
		Short: "Controlla lo stato dello scrutinio: in_corso, parziale N/M sezioni, o completo.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli stato Agrigento
  elezioni-sicilia-pp-cli stato --provincia AG --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[{"comune":"Agrigento","provincia":"AG","stato":"completo"}]`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: ReportCandidati pages to check scrutinio state")
				return nil
			}

			// Resolve comuni to check
			var comuni []scraper.Comune
			if len(args) > 0 {
				c, err := scraper.ResolveComune(args[0], provincia, anno)
				if err != nil {
					return err
				}
				comuni = []scraper.Comune{*c}
			} else {
				// All comuni for the province (or all provinces)
				province := scraper.Province
				if provincia != "" {
					province = []string{strings.ToUpper(provincia)}
				}
				for _, prov := range province {
					cc, err := scraper.FetchComuni(prov, anno)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "avviso %s: %v\n", prov, err)
						continue
					}
					comuni = append(comuni, cc...)
				}
			}

			var stati []StatoComune
			for _, c := range comuni {
				result, _, err := scraper.FetchCandidati(&c, anno)
				if err != nil {
					stati = append(stati, StatoComune{
						Comune:    c.Nome,
						Provincia: c.Provincia,
						Stato:     "errore",
						Dettaglio: err.Error(),
					})
					continue
				}
				stati = append(stati, StatoComune{
					Comune:    c.Nome,
					Provincia: c.Provincia,
					Stato:     result.Stato,
					Dettaglio: result.Dettaglio,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				data, _ := json.MarshalIndent(stati, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			rows := make([][]string, len(stati))
			for i, s := range stati {
				det := s.Dettaglio
				if det == "" {
					det = "-"
				}
				rows[i] = []string{s.Comune, s.Provincia, string(s.Stato), det}
			}
			return flags.printTable(cmd, []string{"COMUNE", "PROV", "STATO", "DETTAGLIO"}, rows)
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Filtra per provincia")
	return cmd
}
