// pp:client-call
// Top-level `search` command (and the shared flag set reused by
// `provvedimenti cerca`). Calls the hand-written gaclient core.
package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

// searchFlags holds the bound values for a provvedimenti search.
type searchFlags struct {
	testo   string
	all     string
	any     string
	not     string
	phrase  string
	tipo    string
	sede    string
	anno    int
	numero  int
	nrg     int
	annoNrg int
	limit   int
}

// addSearchFlags binds the common search flags onto a command.
func addSearchFlags(cmd *cobra.Command, f *searchFlags) {
	cmd.Flags().StringVar(&f.testo, "testo", "", "Ricerca full-text libera (testo semplice).")
	cmd.Flags().StringVar(&f.all, "all", "", "Ricerca avanzata: tutte queste parole (AND).")
	cmd.Flags().StringVar(&f.any, "any", "", "Ricerca avanzata: una qualsiasi di queste parole (OR).")
	cmd.Flags().StringVar(&f.not, "not", "", "Ricerca avanzata: nessuna di queste parole (NOT).")
	cmd.Flags().StringVar(&f.phrase, "phrase", "", "Ricerca avanzata: frase esatta.")
	cmd.Flags().StringVar(&f.tipo, "tipo", "", "Tipo: sentenza, ordinanza, decreto, parere, plenaria, generale.")
	cmd.Flags().StringVar(&f.sede, "sede", "", "Sede: roma, milano, consiglio-di-stato, cgars, ... (28 TAR + CdS).")
	cmd.Flags().IntVar(&f.anno, "anno", 0, "Anno del provvedimento.")
	cmd.Flags().IntVar(&f.numero, "numero", 0, "Numero del provvedimento.")
	cmd.Flags().IntVar(&f.nrg, "nrg", 0, "Numero di registro generale (ricorso).")
	cmd.Flags().IntVar(&f.annoNrg, "anno-nrg", 0, "Anno del registro generale (per la ricerca per NRG).")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Max risultati da scaricare (0 = predefinito del comando; paginazione automatica).")
}

func (f *searchFlags) opts(positional string) gaclient.SearchOptions {
	testo := f.testo
	if testo == "" {
		testo = positional
	}
	return gaclient.SearchOptions{
		Testo: testo, All: f.all, Any: f.any, Not: f.not, Phrase: f.phrase,
		Tipo: f.tipo, Sede: f.sede, Anno: f.anno, Numero: f.numero,
		Nrg: f.nrg, AnnoNrg: f.annoNrg, Limit: f.limit,
	}
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "search [testo]",
		Short: "Cerca sentenze, ordinanze, decreti e pareri (full-text + filtri).",
		Long: "Cerca provvedimenti della Giustizia Amministrativa (TAR, Consiglio di Stato, CGARS).\n" +
			"Ogni risultato porta ECLI e l'URL pubblico del testo integrale.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli search "appalto soccorso istruttorio" --tipo sentenza --sede roma --limit 10
  giustizia-amministrativa-pp-cli search --all "clausola sociale" --json --select ecli,tipo,sede,url
  giustizia-amministrativa-pp-cli search --nrg 422 --anno-nrg 2026 --sede roma`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pos := ""
			if len(args) > 0 {
				pos = args[0]
			}
			return runGASearch(cmd, flags, f.opts(pos))
		},
	}
	addSearchFlags(cmd, &f)
	return cmd
}
