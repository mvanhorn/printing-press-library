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
				return nil
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
			counts := map[string]int{}
			for _, p := range res.Items {
				counts[dimKey(p, dims)]++
			}
			type row struct {
				Key   string `json:"key"`
				Count int    `json:"count"`
			}
			rows := make([]row, 0, len(counts))
			for k, v := range counts {
				rows = append(rows, row{Key: k, Count: v})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Count != rows[j].Count {
					return rows[i].Count > rows[j].Count
				}
				return rows[i].Key < rows[j].Key
			})
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Totale risultati per la query: %d. Distribuzione su un campione di %d (per %s).\n",
					res.Total, len(res.Items), strings.Join(dims, "+"))
			}
			out := map[string]any{
				"total_results": res.Total,
				"sample_size":   len(res.Items),
				"by":            dims,
				"distribution":  rows,
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
