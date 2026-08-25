// pp:client-call
package cli

import "github.com/spf13/cobra"

func newCommissioniSommariCmd(flags *rootFlags) *cobra.Command {
	// Non sono registrati --presidente (il form /bd/ non ha quel campo) né i
	// flag ISIS (--frase, --isis-query, --escludi): l'archivio sta su /bd/ in
	// modo incondizionato, quindi erano criteri che potevano solo fallire, e
	// intanto comparivano in --help e nella superficie MCP. --argomento resta
	// perché è aliasato sul full-text, che il backend ha davvero.
	var (
		flagLegisl   int
		flagAnno     int
		flagNumero   int
		flagCodcom   string
		flagCommis   string
		flagData     string
		flagArgom    string
		flagTesto    string
		flagLimit    int
		flagMaxPages int
	)
	cmd := &cobra.Command{
		Use:         "sommari",
		Args:        rejectPositionalArgs,
		Short:       "Sommari dei lavori delle Commissioni.",
		Example:     "  ars-sicilia-pp-cli commissioni sommari --legisl 18 --commissione \"Bilancio\" --json",
		Annotations: map[string]string{"pp:endpoint": "commissioni.sommari", "mcp:read-only": "true"},
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
			if flagCodcom != "" {
				params["codcom"] = flagCodcom
			}
			if flagCommis != "" {
				params["commissione"] = flagCommis
			}
			if flagData != "" {
				params["data"] = flagData
			}
			if flagArgom != "" {
				params["testo"] = flagArgom
			}
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			return runCerca(cmd, flags, "sommari", cercaParams{
				Params: params,
				Limit:  flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno della seduta (es. 2026). Filtro nativo del backend /bd/.")
	// Il filtro più stretto che il backend /bd/ offre su questo archivio, e per
	// questo il più affidabile: il portale tronca le risposte grandi, e una
	// seduta sola sta in una pagina che arriva sempre intera. Il campo esiste da
	// sempre nel form ($Iseduta_numero) e la specifica lo mappava già: mancava
	// solo il flag, e senza di esso l'unica ricerca sicura era irraggiungibile.
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero della seduta di commissione (es. 270).")
	cmd.Flags().StringVar(&flagCodcom, "codcom", "", "Codice commissione 1-6 (PRIMA..SESTA); in alternativa usa --commissione.")
	cmd.Flags().StringVar(&flagCommis, "commissione", "", "Nome commissione.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data seduta (YYYY-MM-DD; range con YYYY-MM-DD:YYYY-MM-DD).")
	cmd.Flags().StringVar(&flagArgom, "argomento", "", "Argomento (free-text; stesso campo di --testo).")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale sul contenuto della seduta (campo full-text del backend /bd/).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime (0 = auto).")
	return cmd
}
