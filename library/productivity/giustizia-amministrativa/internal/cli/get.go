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
	var format, sede, nrg, file, id string
	var frontMatter, meta bool

	cmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Scarica il testo completo di un provvedimento e lo restituisce in Markdown pulito.",
		Long: "Recupera il testo integrale di una sentenza/ordinanza/decreto/parere (per ECLI o idprovv,\n" +
			"da una ricerca precedente) e lo restituisce in Markdown pulito. Usa --format per text/html/json,\n" +
			"oppure --sede/--nrg/--file per il fetch diretto senza ricerca.\n" +
			"L'output md/text è preceduto per default da un blocco YAML con i metadati\n" +
			"(ecli, sede, sezione, numero, anno, nrg, data_deposito, formato, url) — la fonte resta sempre\n" +
			"risalibile anche quando il testo è troppo lungo per starci tutto. Usa --front-matter=false\n" +
			"per il solo corpo del testo. Con --meta il blocco porta anche i metadati di registro\n" +
			"(oggetto, presidente, estensore, urn NIR, omissis) letti dalla forma XML del documento.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --format md
  giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --front-matter=false
  giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --json
  giustizia-amministrativa-pp-cli get --sede tar_rm --nrg 202600422 --file 202611307_01.html`, "\n"),
		// pp:happy-args da argomenti veri al posizionale, altrimenti il gate phase5
		// segna il comando come 'hollow coverage': mai eseguito davvero nella
		// matrice live. Serve la forma autosufficiente --sede/--nrg/--file: il solo
		// ECLI richiede che il provvedimento sia gia' nello store locale, che su una
		// macchina pulita e' vuoto, e il comando esce 1. Tutto per flag, --id
		// compreso: il posizionale qui e' dichiarato `get [id]` fra quadre e non
		// viene raccolto, a differenza del `run <nome>` di watch.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--id=ECLI:IT:CDS:2026:6289SENT;--sede=cds;--nrg=202601334;--file=202606289_11.html"},
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
	// The MCP mirror of this command has no way to declare a positional
	// argument: the walker emits a schema from the flags plus a generic
	// `args` string. An agent reading that schema reaches for `id` — the name
	// the typed provvedimenti_get tool uses and the one the description keeps
	// repeating — and gets `unknown flag: --id`. Accepting --id as an alias
	// for the positional puts the ECLI back in the mirror's schema, where the
	// agent already looks for it.
	cmd.Flags().StringVar(&id, "id", "", "ECLI o idprovv del provvedimento; equivale all'argomento posizionale.")
	cmd.Flags().StringVar(&format, "format", "md", "Formato di output: md, text, html, json.")
	cmd.Flags().StringVar(&sede, "sede", "", "Schema sede (es. tar_rm) per il fetch diretto senza ricerca.")
	cmd.Flags().StringVar(&nrg, "nrg", "", "NRG per il fetch diretto.")
	cmd.Flags().StringVar(&file, "file", "", "nomeFile per il fetch diretto (es. 202611307_01.html).")
	cmd.Flags().BoolVar(&meta, "meta", false, "Aggiungi i metadati di registro letti dalla forma XML del documento (oggetto, presidente, estensore, urn NIR della sezione, flag omissis). Costa una seconda richiesta al portale; i provvedimenti pubblicati in PDF non ne hanno.")
	cmd.Flags().BoolVar(&frontMatter, "front-matter", true, "Anteponi un blocco YAML con i metadati, url incluso (solo output md/text). --front-matter=false per il solo testo.")
	return cmd
}
