// pp:client-call
// Real implementation: delegates to the shared gaclient core. `provvedimenti
// get` and the top-level `get` resolve a provvedimento and return its full text.
package cli

import (
	"github.com/spf13/cobra"
)

func newProvvedimentiGetCmd(flags *rootFlags) *cobra.Command {
	var format, sede, nrg, file, id string
	var frontMatter, meta bool
	cmd := &cobra.Command{
		Use:         "get [id]",
		Short:       "Scarica il testo integrale di un provvedimento (per ECLI o idprovv).",
		Example:     "  giustizia-amministrativa-pp-cli provvedimenti get IT:TARLAZ:2026:11307SENT --format md",
		Annotations: map[string]string{"pp:endpoint": "provvedimenti.get", "pp:method": "GET", "pp:path": "/visualizzah2/", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				id = args[0]
			}
			if id == "" && sede == "" {
				return cmd.Help()
			}
			return runGAGet(cmd, flags, id, format, sede, nrg, file, frontMatter, meta)
		},
	}
	// Same alias as the top-level `get`, so the two spellings of the same
	// command don't disagree on how the ECLI is passed.
	cmd.Flags().StringVar(&id, "id", "", "ECLI o idprovv del provvedimento; equivale all'argomento posizionale.")
	cmd.Flags().StringVar(&format, "format", "md", "Formato di output: md, text, html, json.")
	cmd.Flags().StringVar(&sede, "sede", "", "Schema sede (es. tar_rm) per il fetch diretto senza ricerca.")
	cmd.Flags().StringVar(&nrg, "nrg", "", "NRG per il fetch diretto.")
	cmd.Flags().StringVar(&file, "file", "", "nomeFile per il fetch diretto (es. 202611307_01.html).")
	cmd.Flags().BoolVar(&meta, "meta", false, "Aggiungi i metadati di registro letti dalla forma XML del documento (oggetto, presidente, estensore, urn NIR della sezione, flag omissis). Costa una seconda richiesta al portale; i provvedimenti pubblicati in PDF non ne hanno.")
	cmd.Flags().BoolVar(&frontMatter, "front-matter", true, "Anteponi un blocco YAML con i metadati, url incluso (solo output md/text). --front-matter=false per il solo testo.")
	return cmd
}
