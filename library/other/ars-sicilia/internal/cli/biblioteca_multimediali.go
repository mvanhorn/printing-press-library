// pp:client-call
package cli

import "github.com/spf13/cobra"

func newBibliotecaMultimedialiCmd(flags *rootFlags) *cobra.Command {
	var (
		flagTitolo   string
		flagAutore   string
		flagISIS     string
		flagLimit    int
		flagMaxPages int
	)
	cmd := &cobra.Command{
		Use:         "multimediali",
		Short:       "Cerca nelle Opere Multimediali (archivio 205multimedia).",
		Example:     "  ars-sicilia-pp-cli biblioteca multimediali --titolo \"Falcone\" --json",
		Annotations: map[string]string{"pp:endpoint": "biblioteca.multimediali", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagTitolo != "" {
				params["titolo"] = flagTitolo
			}
			if flagAutore != "" {
				params["autore"] = flagAutore
			}
			return runCerca(cmd, flags, "biblioteca", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().StringVar(&flagTitolo, "titolo", "", "Titolo dell'opera.")
	cmd.Flags().StringVar(&flagAutore, "autore", "", "Autore.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime (0 = auto).")
	return cmd
}
