// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newDdlCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl     int
		flagAnno       int
		flagFirmatario string
		flagMateria    string
		flagIter       string
		flagTesto      string
		flagISIS       string
		flagLimit      int
		flagMaxPages   int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
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
			return runCerca(cmd, flags, "ddl", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18).")
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno di presentazione.")
	cmd.Flags().StringVar(&flagFirmatario, "firmatario", "", "Nome o cognome del firmatario.")
	cmd.Flags().StringVar(&flagMateria, "materia", "", "Materia/settore.")
	cmd.Flags().StringVar(&flagIter, "iter", "", "Stato dell'iter.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale libera.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	return cmd
}
