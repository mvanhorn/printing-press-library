// pp:client-call
package cli

import "github.com/spf13/cobra"

func newBibliotecaCercaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagAutore   string
		flagTitolo   string
		flagSoggetto string
		flagISBN     string
		flagDewey    string
		flagISIS     string
		flagLimit    int
		flagMaxPages int
	)
	cmd := &cobra.Command{
		Use:         "cerca",
		Short:       "Cerca nel Catalogo Bibliografico (archivio 205).",
		Example:     "  ars-sicilia-pp-cli biblioteca cerca --autore \"Sciascia\" --json",
		Annotations: map[string]string{"pp:endpoint": "biblioteca.cerca", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if flagAutore != "" {
				params["autore"] = flagAutore
			}
			if flagTitolo != "" {
				params["titolo"] = flagTitolo
			}
			if flagSoggetto != "" {
				params["soggetto"] = flagSoggetto
			}
			if flagISBN != "" {
				params["isbn"] = flagISBN
			}
			if flagDewey != "" {
				params["dewey"] = flagDewey
			}
			return runCerca(cmd, flags, "biblioteca", cercaParams{
				Params: params, ISISRaw: flagISIS,
				Limit: flagLimit, MaxPages: flagMaxPages,
			})
		},
	}
	cmd.Flags().StringVar(&flagAutore, "autore", "", "Autore (cognome nome).")
	cmd.Flags().StringVar(&flagTitolo, "titolo", "", "Titolo dell'opera.")
	cmd.Flags().StringVar(&flagSoggetto, "soggetto", "", "Soggetto/materia.")
	cmd.Flags().StringVar(&flagISBN, "isbn", "", "ISBN.")
	cmd.Flags().StringVar(&flagDewey, "dewey", "", "Classificazione Dewey.")
	cmd.Flags().StringVar(&flagISIS, "isis-query", "", "Espressione ISIS grezza (escape hatch).")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max risultati da scaricare.")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Pagine massime (0 = auto).")
	return cmd
}
