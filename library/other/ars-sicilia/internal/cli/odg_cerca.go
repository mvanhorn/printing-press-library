// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newOdgCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl     int
		flagFirmatario string
		flagRubrica    string
		flagTesto      string
		flagISIS       string
		flagLimit      int
		flagMaxPages   int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Short:   "Cerca ordini del giorno.",
		Example: "  ars-sicilia-pp-cli odg cerca --legisl 18 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "odg.cerca",
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
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			return runCerca(cmd, flags, "odg", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().StringVar(&flagFirmatario, "firmatario", "", "Firmatario.")
	cmd.Flags().StringVar(&flagRubrica, "rubrica", "", "Rubrica.")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	return cmd
}
