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
		flagISIS     string
		flagLimit    int
		flagMaxPages int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
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
			return runCerca(cmd, flags, "leggi", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18 per XVIII).")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno della legge.")
	cmd.Flags().IntVar(&flagNumero, "numero", 0, "Numero della legge.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale libera.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza che bypassa la traduzione automatica dei flag (escape hatch power-user).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	return cmd
}
