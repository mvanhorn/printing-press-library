// pp:client-call
// pp:data-source live
// Novel feature: run the portal's "verifica appello" in batch over a search and
// report which first-instance (TAR) rulings were appealed. The form exposes this
// one ruling at a time; we loop it and assemble the result.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

func newNovelAppealChainCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "appeal-chain",
		Short: "Verifica in batch quali sentenze TAR sono state appellate (TAR -> Consiglio di Stato).",
		Long: "Esegue una ricerca e per ogni provvedimento di primo grado interroga 'verifica appello'\n" +
			"del portale, riportando se e da quali atti è stato appellato.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli appeal-chain --testo "project financing" --tipo sentenza --limit 20
  giustizia-amministrativa-pp-cli appeal-chain --testo appalto --sede roma --json`, "\n"),
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
			type chain struct {
				Ecli     string            `json:"ecli"`
				Tipo     string            `json:"tipo"`
				Sede     string            `json:"sede"`
				Nrg      string            `json:"nrg"`
				URL      string            `json:"url"`
				Appealed bool              `json:"appealed"`
				Appeals  []json.RawMessage `json:"appeals,omitempty"`
			}
			out := []chain{}
			appealedCount := 0
			for _, p := range res.Items {
				appeals, total, aerr := c.VerificaAppello(cmd.Context(), p)
				if aerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "verifica appello fallita per %s: %v\n", provID(p), aerr)
					continue
				}
				ch := chain{Ecli: p.Ecli, Tipo: p.Tipo, Sede: p.Sede, Nrg: p.Nrg, URL: p.URL, Appealed: total > 0, Appeals: appeals}
				if ch.Appealed {
					appealedCount++
				}
				out = append(out, ch)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Esaminati %d provvedimenti: %d risultano appellati.\n", len(out), appealedCount)
			}
			data, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addSearchFlags(cmd, &f)
	return cmd
}
