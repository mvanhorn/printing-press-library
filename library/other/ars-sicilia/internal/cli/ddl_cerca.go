// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newDdlCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl     int
		flagAnno       int
		flagData       string
		flagNumero     int
		flagFirmatario string
		flagMateria    string
		flagIter       string
		flagTesto      string
		flagFrase      string
		flagISIS       string
		flagLimit      int
		flagMaxPages   int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Args:    rejectPositionalArgs,
		Short:   "Cerca disegni di legge per legislatura, anno, firmatario o materia.",
		Example: "  ars-sicilia-pp-cli ddl cerca --legisl 18 --anno 2024 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "ddl.cerca",
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
			if flagFirmatario != "" {
				params["firmatario"] = flagFirmatario
			}
			if flagMateria != "" {
				params["materia"] = flagMateria
			}
			if flagIter != "" {
				params["iter"] = flagIter
			}
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			if flagFrase != "" {
				params["frase"] = flagFrase
			}
			return runCerca(cmd, flags, "ddl", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18).")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno di presentazione (qualificato su DATPRE come range 1 gen - 31 dic, non testo libero).")
	cmd.Flags().StringVar(&flagData, "data", "", "Data di presentazione (YYYY-MM-DD; range con YYYY-MM-DD:YYYY-MM-DD). Stesso campo DATPRE di --anno, ma con estremi liberi: per un anno intero basta --anno.")
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero del DDL (campo NUMDDL; per gli stralci è l'ID numerico interno, non la sigla \"N/A Stralcio\" — vedi il campo excerpt per la designazione ufficiale).")
	cmd.Flags().StringVar(&flagFirmatario, "firmatario", "", "Nome o cognome del firmatario.")
	cmd.Flags().StringVar(&flagMateria, "materia", "", "Materia/settore.")
	cmd.Flags().StringVar(&flagIter, "iter", "", "Stato dell'iter.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale libera.")
	cmd.Flags().StringVar(&flagFrase, "frase", "", "Cerca le parole come locuzione, adiacenti e nell'ordine dato (ISIS adj). Piu' preciso di --testo, che combina le parole in AND sull'intero documento: --testo \"aree idonee\" aggancia anche chi ha le due parole in articoli diversi.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	cmd.Flags().String("escludi", "", "Escludi i documenti che contengono questo termine (ISIS NOT).")
	cmd.Flags().Bool("con-firmatari", false, "Includi l'elenco completo dei firmatari per ogni risultato: apre il documento di ogni riga (una richiesta in piu' per riga, piu' lento). Senza questo flag la lista mostra solo il primo firmatario, come il portale.")
	// --anno e --data qualificano lo stesso campo DATPRE: insieme diventerebbero
	// due range in AND sullo stesso campo, cioe' zero risultati silenziosi invece
	// dell'intersezione che l'utente si aspetta.
	cmd.MarkFlagsMutuallyExclusive("anno", "data")
	return cmd
}
