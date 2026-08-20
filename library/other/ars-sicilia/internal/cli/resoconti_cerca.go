// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newResocontiCercaCmd(flags *rootFlags) *cobra.Command {
	// I flag ISIS (--argomento, --frase, --isis-query, --escludi) non sono
	// registrati: questo archivio è servito dal backend /bd/ in modo
	// incondizionato (client.go: solo `get` forza Icaro, e non c'è flag utente
	// per chiederlo), e /bd/ ha un form fisso senza equivalente per quei filtri.
	// Registrarli significava pubblicizzare in --help e nella superficie MCP dei
	// criteri che potevano solo fallire. La ricerca testuale è --testo ($TTEXT).
	var (
		flagLegisl   int
		flagAnno     int
		flagData     string
		flagNumero   int
		flagOratore  string
		flagTesto    string
		flagLimit    int
		flagMaxPages int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Args:    rejectPositionalArgs,
		Short:   "Cerca resoconti delle sedute d'aula per data, numero, oratore o testo.",
		Example: "  ars-sicilia-pp-cli resoconti cerca --legisl 18 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "resoconti.cerca",
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
			if flagData != "" {
				params["data"] = flagData
			}
			if flagNumero != 0 {
				params["numero"] = itoa(flagNumero)
			}
			if flagOratore != "" {
				params["oratore"] = flagOratore
			}
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			return runCerca(cmd, flags, "resoconti", cercaParams{
				Params: params,
				Limit:  flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno della seduta.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data seduta (YYYY-MM-DD; range con YYYY-MM-DD:YYYY-MM-DD).")
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero seduta.")
	cmd.Flags().StringVar(&flagOratore, "oratore", "", "Cognome/nome dell'oratore: filtra le sedute in cui è intervenuto (risolto sull'anagrafica del portale; se combinato con --legisl considera solo chi vi è attivo).")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale sul contenuto della seduta (campo full-text del backend /bd/).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	return cmd
}
