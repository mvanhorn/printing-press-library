// pp:client-call
// pp:data-source live
// Novel feature: extract the "principio di diritto"/massima paragraphs from a
// corpus of rulings into a single digest. Heuristic: scans the full text for
// the paragraphs around recognised legal-maxim markers. Quality depends on the
// document's structure (clearly labelled as a heuristic in the help).
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

var reMassimaMarker = regexp.MustCompile(`(?i)(principio di diritto|deve essere affermato il (?:seguente )?principio|enuncia il (?:seguente )?principio|massima)`)

func newNovelMassimeCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "massime",
		Short: "Estrae i paragrafi 'principio di diritto'/massima da un corpus (euristico).",
		Long: "Esegue una ricerca, scarica i testi integrali ed estrae i paragrafi che contengono i\n" +
			"marcatori dei principi di diritto/massime. È un'euristica: la resa dipende dalla struttura\n" +
			"del provvedimento.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli massime --testo "clausola sociale" --limit 20
  giustizia-amministrativa-pp-cli massime --all "soccorso istruttorio" --tipo sentenza --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if gaSkip(flags) {
				return nil
			}
			opts := f.opts("")
			if !hasAnySearchInput(opts) {
				return fmt.Errorf("specifica almeno un criterio di ricerca (--testo, --all, ...)")
			}
			if opts.Limit == 0 {
				opts.Limit = 20
			}
			c := gaclient.New()
			res, err := c.Search(cmd.Context(), opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			st, _ := openGAStore(cmd.Context())
			if st != nil {
				defer st.Close()
			}
			type digest struct {
				Ecli    string   `json:"ecli"`
				Tipo    string   `json:"tipo"`
				Sede    string   `json:"sede"`
				URL     string   `json:"url"`
				Massime []string `json:"massime"`
			}
			out := []digest{}
			for _, p := range res.Items {
				docHTML, ferr := c.FullText(cmd.Context(), p)
				if ferr != nil {
					continue
				}
				if p.DataDeposito == "" {
					p.DataDeposito = gaclient.ExtractDataDeposito(docHTML)
				}
				p.FullText = gaclient.HTMLToMarkdown(docHTML)
				if st != nil {
					persistProvvedimenti(st, []gaclient.Provvedimento{p})
				}
				found := extractMassime(p.FullText)
				if len(found) > 0 {
					out = append(out, digest{Ecli: p.Ecli, Tipo: p.Tipo, Sede: p.Sede, URL: p.URL, Massime: found})
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Massime estratte da %d provvedimenti (su %d esaminati).\n", len(out), len(res.Items))
			}
			data, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addSearchFlags(cmd, &f)
	return cmd
}

// extractMassime returns paragraphs containing a legal-maxim marker.
func extractMassime(text string) []string {
	var out []string
	for _, para := range strings.Split(text, "\n\n") {
		p := strings.TrimSpace(para)
		if p != "" && reMassimaMarker.MatchString(p) {
			out = append(out, p)
		}
	}
	return out
}
