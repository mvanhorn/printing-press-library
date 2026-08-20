package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string
	var intervallo time.Duration
	var nVolte int
	var nSet bool

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Polling periodico dello stato scrutini per tutti i comuni.",
		Long: `Esegue polling periodico dello stato degli scrutini e mostra
aggiornamenti ogni volta che cambia lo stato di un comune.
Interrompi con Ctrl+C.`,
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli watch
  elezioni-sicilia-pp-cli watch --intervallo 5m --provincia PA
  elezioni-sicilia-pp-cli watch --n 3 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"ciclo":1,"aggiornamenti":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "would poll every %s for scrutinio updates (province: %s)\n",
					intervallo, ifEmpty(provincia, "tutte"))
				return nil
			}

			// In non-interactive / automated mode without explicit --n: exit with a notice.
			// watch is designed for terminal sessions; CI/agents should use 'stato' for snapshots.
			nonInteractive := flags.noInput || !isTerminal(cmd.OutOrStdout()) ||
				cliutil.IsVerifyEnv() || os.Getenv("CI") != "" || os.Getenv("TERM") == ""
			if !nSet && nonInteractive {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), `{"nota":"watch richiede sessione interattiva; usa 'stato' per snapshot","avviare_con":"--n 1"}`)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "watch richiede sessione interattiva. Per un singolo snapshot: stato --json")
				}
				return nil
			}

			province := scraper.Province
			if provincia != "" {
				province = []string{strings.ToUpper(provincia)}
			}

			// Initial state
			prevStato := map[string]string{}
			ciclo := 0
			maxCicli := nVolte

			for {
				ciclo++
				aggiornamenti := make([]map[string]string, 0)

				for _, prov := range province {
					comuni, err := scraper.FetchComuni(prov, anno)
					if err != nil {
						continue
					}
					for _, c := range comuni {
						result, _, err := scraper.FetchCandidati(&c, anno)
						if err != nil {
							continue
						}
						key := fmt.Sprintf("%s%s", prov, c.Codice)
						statoStr := string(result.Stato)
						if result.Dettaglio != "" {
							statoStr += " (" + result.Dettaglio + ")"
						}
						if prev, ok := prevStato[key]; !ok || prev != statoStr {
							prevStato[key] = statoStr
							if ok { // only report changes (skip initial population)
								aggiornamenti = append(aggiornamenti, map[string]string{
									"comune":    c.Nome,
									"provincia": prov,
									"da":        prev,
									"a":         statoStr,
								})
							}
						}
					}
				}

				if flags.asJSON {
					out := map[string]any{
						"ciclo":         ciclo,
						"timestamp":     time.Now().Format(time.RFC3339),
						"aggiornamenti": aggiornamenti,
					}
					data, _ := json.Marshal(out)
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				} else if len(aggiornamenti) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %d aggiornamenti:\n",
						time.Now().Format("15:04:05"), len(aggiornamenti))
					for _, a := range aggiornamenti {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s): %s → %s\n",
							a["comune"], a["provincia"], a["da"], a["a"])
					}
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] Nessun aggiornamento (ciclo %d)\n",
						time.Now().Format("15:04:05"), ciclo)
				}

				if maxCicli > 0 && ciclo >= maxCicli {
					break
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Prossimo aggiornamento tra %s (più il tempo di scraping)...\n", intervallo)
				time.Sleep(intervallo)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Filtra per provincia")
	cmd.Flags().DurationVar(&intervallo, "intervallo", 5*time.Minute, "Intervallo tra i polling (es. 2m, 10m)")
	cmd.Flags().IntVar(&nVolte, "n", 0, "Numero di cicli (0 = infinito, Ctrl+C per fermare)")
	cmd.Flags().BoolVar(&nSet, "n-set", false, "")
	_ = cmd.Flags().MarkHidden("n-set")
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		nSet = c.Flags().Changed("n")
		return nil
	}
	return cmd
}

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
