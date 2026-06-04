// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import "github.com/spf13/cobra"

func newPareriCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl      int
		flagCommissione string
		flagOggetto     string
		flagISIS        string
		flagLimit       int
		flagMaxPages    int
	)

	cmd := &cobra.Command{
		Use:     "cerca",
		Short:   "Cerca pareri richiesti dal Governo regionale.",
		Example: "  ars-sicilia-pp-cli pareri cerca --legisl 18 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "pareri.cerca",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagLegisl != 0 {
				params["legisl"] = itoa(flagLegisl)
			}
			if flagCommissione != "" {
				params["commissione"] = flagCommissione
			}
			if flagOggetto != "" {
				params["oggetto"] = flagOggetto
			}
			return runCerca(cmd, flags, "pareri", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().StringVar(&flagCommissione, "commissione", "", "Commissione competente.")
	cmd.Flags().StringVar(&flagOggetto, "oggetto", "", "Oggetto del parere (free-text).")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime da scaricare (0 = auto da --limit).")
	return cmd
}
