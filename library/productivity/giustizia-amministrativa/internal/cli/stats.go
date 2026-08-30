// pp:client-call
// pp:data-source live
// Novel feature: distribution of a theme by sede/sezione/tipo/anno. Aggregates
// over a fetched sample and reports the grand total separately (honest about
// the sample size — the form returns a flat list + count, never a breakdown).
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	var by string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Distribuzione di un tema per sede, sezione, tipo o anno (su un campione).",
		Long: "Esegue una ricerca e raggruppa i risultati per le dimensioni indicate in --by.\n" +
			"L'aggregazione è calcolata sul campione scaricato (--limit); il totale complessivo\n" +
			"riportato dal portale è mostrato a parte.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli stats --testo appalto --by sede,anno --limit 200
  giustizia-amministrativa-pp-cli stats --testo "soccorso istruttorio" --by tipo --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if gaSkip(flags) {
				return emitSkip(cmd, flags)
			}
			dims := splitDims(by)
			if len(dims) == 0 {
				dims = []string{"tipo"}
			}
			opts := f.opts("")
			if !hasAnySearchInput(opts) {
				return fmt.Errorf("specifica almeno un criterio di ricerca (--testo, --all, ...)")
			}
			if opts.Limit == 0 {
				opts.Limit = 200
			}
			c := gaclient.New()
			res, err := c.Search(cmd.Context(), opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s\n", w)
			}
			// Under a sede sweep the sample holds an equal quota per sede by
			// construction, so counting its rows by sede measures the quota,
			// not the country: every sede comes out with the same figure, and
			// against the real totals the ranking can even invert. It looks
			// like a distribution and is an artefact — the worst shape for a
			// number someone may put in a filing. The sweep already collected
			// each sede's declared total, so report those instead.
			counts := map[string]int{}
			fromTotals := false
			if len(dims) == 1 && dims[0] == "sede" && len(res.TotalsBySede) > 0 {
				for sede, n := range res.TotalsBySede {
					if n > 0 {
						counts[strings.ToUpper(sede)] = n
					}
				}
				fromTotals = true
			} else {
				for _, p := range res.Items {
					counts[dimKey(p, dims)]++
				}
			}
			// When the sample stops short of the declared total, the buckets are
			// not equally trustworthy: the portal returns results in a fixed
			// order, so every bucket before the cut is complete and the one
			// holding the last returned item is sliced through the middle —
			// while anything that would have come after it is missing outright.
			// Reporting all of them as plain numbers is what turns "103
			// provvedimenti in 2023" into "53", with nothing to show it happened.
			troncato := ""
			if !fromTotals && len(res.Items) > 0 && res.Total > len(res.Items) {
				troncato = dimKey(res.Items[len(res.Items)-1], dims)
			}
			type row struct {
				Key      string `json:"key"`
				Count    int    `json:"count"`
				Troncato bool   `json:"troncato,omitempty"`
			}
			rows := make([]row, 0, len(counts))
			for k, v := range counts {
				rows = append(rows, row{Key: k, Count: v, Troncato: k == troncato})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Count != rows[j].Count {
					return rows[i].Count > rows[j].Count
				}
				return rows[i].Key < rows[j].Key
			})
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if fromTotals {
					fmt.Fprintf(cmd.ErrOrStderr(), "Totale risultati per la query: %d. Distribuzione sui totali dichiarati dal portale per ciascuna sede (non su un campione).\n", res.Total)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Totale risultati per la query: %d. Distribuzione su un campione di %d (per %s).\n",
						res.Total, len(res.Items), strings.Join(dims, "+"))
				}
			}
			if troncato != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s\n",
					avvisoGruppoTroncato(len(res.Items), res.Total, troncato, dims))
			}
			out := map[string]any{
				"gruppo_troncato": troncato,
				"total_results":   res.Total,
				"sample_size":     len(res.Items),
				"conteggi_da":     map[bool]string{true: "totali dichiarati dal portale per sede", false: "campione scaricato"}[fromTotals],
				"by":              dims,
				"distribution":    rows,
			}
			data, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addSearchFlags(cmd, &f)
	cmd.Flags().StringVar(&by, "by", "tipo", "Dimensioni di raggruppamento separate da virgola: sede, sezione, tipo, anno.")
	return cmd
}

func splitDims(by string) []string {
	var out []string
	for _, d := range strings.Split(by, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func dimKey(p gaclient.Provvedimento, dims []string) string {
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		switch d {
		case "sede":
			parts = append(parts, orNA(p.Sede))
		case "sezione":
			parts = append(parts, orNA(p.Sezione))
		case "tipo":
			parts = append(parts, orNA(p.Tipo))
		case "anno":
			if p.Anno != 0 {
				parts = append(parts, strconv.Itoa(p.Anno))
			} else {
				parts = append(parts, "N/A")
			}
		default:
			parts = append(parts, "?")
		}
	}
	return strings.Join(parts, " | ")
}

func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return "N/A"
	}
	return s
}
