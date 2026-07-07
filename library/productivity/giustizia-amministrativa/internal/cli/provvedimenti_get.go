// pp:client-call
// Real implementation: delegates to the shared gaclient core. `provvedimenti
// get` and the top-level `get` resolve a provvedimento and return its full text.
package cli

import (
	"github.com/spf13/cobra"
)

func newProvvedimentiGetCmd(flags *rootFlags) *cobra.Command {
	var format, sede, nrg, file string
	cmd := &cobra.Command{
		Use:         "get [id]",
		Short:       "Scarica il testo integrale di un provvedimento (per ECLI o idprovv).",
		Example:     "  giustizia-amministrativa-pp-cli provvedimenti get IT:TARLAZ:2026:11307SENT --format md",
		Annotations: map[string]string{"pp:endpoint": "provvedimenti.get", "pp:method": "GET", "pp:path": "/visualizzah2/", "mcp:read-only": "true"},
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
