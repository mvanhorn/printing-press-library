// pp:client-call
// pp:data-source auto
// Flagship novel feature: fetch a provvedimento's full text and render it as
// clean Markdown (default), text, HTML, or JSON. Delegates to the gaclient core.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelGetCmd(flags *rootFlags) *cobra.Command {
	var format, sede, nrg, file string

	cmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Scarica il testo completo di un provvedimento e lo restituisce in Markdown pulito.",
		Long: "Recupera il testo integrale di una sentenza/ordinanza/decreto/parere (per ECLI o idprovv,\n" +
			"da una ricerca precedente) e lo restituisce in Markdown pulito. Usa --format per text/html/json,\n" +
			"oppure --sede/--nrg/--file per il fetch diretto senza ricerca.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --format md
  giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --json
  giustizia-amministrativa-pp-cli get --sede tar_rm --nrg 202600422 --file 202611307_01.html`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = args[0]
			}
			if id == "" && sede == "" {
				return cmd.Help()
			}
			return runGAGet(cmd, flags, id, format, sede, nrg, file)
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "Formato di output: md, text, html, json.")
	cmd.Flags().StringVar(&sede, "sede", "", "Schema sede (es. tar_rm) per il fetch diretto senza ricerca.")
	cmd.Flags().StringVar(&nrg, "nrg", "", "NRG per il fetch diretto.")
	cmd.Flags().StringVar(&file, "file", "", "nomeFile per il fetch diretto (es. 202611307_01.html).")
	return cmd
}
