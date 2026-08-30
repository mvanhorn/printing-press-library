// pp:client-call
// Replaces generator-emitted stub: real implementation lives in
// internal/icaroclient/. The original `extractHTMLResponse` path could not
// handle the Icaro session+pagination flow.

package cli

import "github.com/spf13/cobra"

func newLeggiCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl   int
		flagAnno     int
		flagNumero   int
		flagTesto    string
		flagFrase    string
		flagISIS     string
		flagLimit    int
		flagMaxPages int
		flagArticoli bool
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Args:    rejectPositionalArgs,
		Short:   "Cerca leggi regionali per legislatura, anno, numero o testo.",
		Example: "  ars-sicilia-pp-cli leggi cerca --legisl 18 --anno 2024 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "leggi.cerca",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagLegisl != 0 {
				params["legisl"] = itoa(flagLegisl)
			}
			if flagAnno != 0 {
				params["anno"] = itoa(flagAnno)
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
			// L'archivio è indicizzato per articolo: senza aggregazione il
			// --limit lo consumano gli articoli della prima legge (vedi
			// leggi_collapse.go). Con --articoli si torna alle righe grezze.
			p := cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			}
			if !flagArticoli {
				p.AggregaLeggi = true
				p.LimitLeggi = flagLimit
				p.Limit = leggiRawLimit(flagLimit)
			}
			return runCerca(cmd, flags, "leggi", p)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18 per XVIII).")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno della legge (filtro temporale di questo archivio: non esiste --data sulle leggi; disambigua anche numeri ripetuti tra anni).")
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero della legge.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale libera.")
	cmd.Flags().StringVar(&flagFrase, "frase", "", "Cerca le parole come locuzione, adiacenti e nell'ordine dato (ISIS adj). Piu' preciso di --testo, che combina le parole in AND sull'intero documento: --testo \"aree idonee\" aggancia anche chi ha le due parole in articoli diversi.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza che bypassa la traduzione automatica dei flag (escape hatch power-user).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max leggi da restituire (con --articoli: max righe-articolo).")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	cmd.Flags().BoolVar(&flagArticoli, "articoli", false, "Una riga per articolo invece che una per legge: mostra quali articoli la ricerca ha agganciato (con --testo dice dove ricorre il termine).")
	cmd.Flags().String("escludi", "", "Escludi i documenti che contengono questo termine (ISIS NOT).")
	return cmd
}
