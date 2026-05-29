// pp:client-call
package cli

import "github.com/spf13/cobra"

func newCommissioniSommariCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl   int
		flagCodcom   string
		flagCommis   string
		flagData     string
		flagPresid   string
		flagArgom    string
		flagTesto    string
		flagISIS     string
		flagLimit    int
		flagMaxPages int
	)
	cmd := &cobra.Command{
		Use:         "sommari",
		Short:       "Sommari dei lavori delle Commissioni.",
		Example:     "  ars-sicilia-pp-cli commissioni sommari --legisl 18 --commissione \"Bilancio\" --json",
		Annotations: map[string]string{"pp:endpoint": "commissioni.sommari", "mcp:read-only": "true"},
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
			if flagPresid != "" {
				params["presidente"] = flagPresid
			}
			if flagArgom != "" {
				params["testo"] = flagArgom
			}
			if flagTesto != "" {
				params["testo"] = flagTesto
			}
			return runCerca(cmd, flags, "sommari", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura.")
	cmd.Flags().StringVar(&flagCodcom, "codcom", "", "Codice commissione.")
	cmd.Flags().StringVar(&flagCommis, "commissione", "", "Nome commissione.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data seduta (YYYY-MM-DD).")
	cmd.Flags().StringVar(&flagPresid, "presidente", "", "Nome del presidente di seduta.")
	cmd.Flags().StringVar(&flagArgom, "argomento", "", "Argomento (free-text).")
	cmd.Flags().StringVar(&flagTesto, "testo", "", "Ricerca testuale.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime (0 = auto).")
	return cmd
}
