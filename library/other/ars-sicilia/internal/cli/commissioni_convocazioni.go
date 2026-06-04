// pp:client-call
package cli

import "github.com/spf13/cobra"

func newCommissioniConvocazioniCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl   int
		flagCodcom   string
		flagCommis   string
		flagData     string
		flagISIS     string
		flagLimit    int
		flagMaxPages int
	)
	cmd := &cobra.Command{
		Use:         "convocazioni",
		Short:       "Convocazioni delle Commissioni.",
		Example:     "  ars-sicilia-pp-cli commissioni convocazioni --legisl 18 --codcom 5 --json",
		Annotations: map[string]string{"pp:endpoint": "commissioni.convocazioni", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagLegisl != 0 {
				params["legisl"] = itoa(flagLegisl)
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
			return runCerca(cmd, flags, "convocazioni", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().StringVar(&flagCodcom, "codcom", "", "Codice numerico commissione.")
	cmd.Flags().StringVar(&flagCommis, "commissione", "", "Nome commissione.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data seduta (YYYY-MM-DD).")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime (0 = auto).")
	return cmd
}
