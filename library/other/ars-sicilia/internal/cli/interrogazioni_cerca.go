// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newInterrogazioniCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl     int
		flagFirmatario string
		flagRubrica    string
		flagData       string
		flagNumero     int
		flagTesto      string
		flagFrase      string
		flagISIS       string
		flagLimit      int
		flagMaxPages   int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Args:    rejectPositionalArgs,
		Short:   "Cerca interrogazioni parlamentari.",
		Example: "  ars-sicilia-pp-cli interrogazioni cerca --legisl 18 --firmatario \"Rossi\" --json",
		Annotations: map[string]string{
			"pp:endpoint":   "interrogazioni.cerca",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagLegisl != 0 {
				params["legisl"] = itoa(flagLegisl)
			}
			if flagFirmatario != "" {
				params["firmatario"] = flagFirmatario
			}
			if flagRubrica != "" {
				params["rubrica"] = flagRubrica
			}
			if flagData != "" {
				params["data"] = flagData
			}
			if flagNumero != 0 {
				params["numero"] = itoa(flagNumero)
			}
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			if flagFrase != "" {
				params["frase"] = flagFrase
			}
			return runCerca(cmd, flags, "interrogazioni", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().StringVar(&flagFirmatario, "firmatario", "", "Firmatario.")
	cmd.Flags().StringVar(&flagRubrica, "rubrica", "", "Rubrica/materia.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data di presentazione (YYYY-MM-DD; range con YYYY-MM-DD:YYYY-MM-DD). Non esiste --anno su questo archivio: per un anno intero usa --data AAAA-01-01:AAAA-12-31.")
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero dell'atto (campo NUMORD). Piu' preciso di --testo: cercare il numero come testo libero aggancia ogni documento che lo cita, e l'atto voluto puo' finire oltre il --limit.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale.")
	cmd.Flags().StringVar(&flagFrase, "frase", "", "Cerca le parole come locuzione, adiacenti e nell'ordine dato (ISIS adj). Piu' preciso di --testo, che combina le parole in AND sull'intero documento: --testo \"aree idonee\" aggancia anche chi ha le due parole in articoli diversi.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	cmd.Flags().String("escludi", "", "Escludi i documenti che contengono questo termine (ISIS NOT).")
	cmd.Flags().Bool("con-firmatari", false, "Includi l'elenco completo dei firmatari per ogni risultato: apre il documento di ogni riga (una richiesta in piu' per riga, piu' lento). Senza questo flag la lista mostra solo il primo firmatario, come il portale.")
	return cmd
}
