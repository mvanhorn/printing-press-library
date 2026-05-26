package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

// RiepilogoProvincia holds summary data for one province.
type RiepilogoProvincia struct {
	Provincia      string `json:"provincia"`
	Comuni         int    `json:"comuni_alle_elezioni"`
	TotaleElettori string `json:"totale_elettori,omitempty"`
	PercAffluenza  string `json:"perc_affluenza_finale,omitempty"`
}

// RiepilogoRegionale holds the full regional summary.
type RiepilogoRegionale struct {
	Anno     int                  `json:"anno"`
	Province []RiepilogoProvincia `json:"province"`
}

func newRiepilogoCmd(flags *rootFlags) *cobra.Command {
	var anno int

	cmd := &cobra.Command{
		Use:   "riepilogo",
		Short: "Affluenza aggregata per tutte le 9 province siciliane in un unico output.",
		Long: `Aggrega i dati di affluenza da tutte le province siciliane
in un riepilogo regionale strutturato.`,
		Example:     "  elezioni-sicilia-pp-cli riepilogo --json\n  elezioni-sicilia-pp-cli riepilogo --json --select province.affluenza",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"anno":2026,"province":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: ReportTabellaAffluenza.html and aggregate by province")
				return nil
			}

			records, _, err := scraper.FetchAffluenza(anno)
			if err != nil {
				return fmt.Errorf("riepilogo: %w", err)
			}

			// Aggregate by province
			provMap := map[string]*RiepilogoProvincia{}
			for _, prov := range scraper.Province {
				provMap[prov] = &RiepilogoProvincia{Provincia: prov}
			}
			type provAccum struct{ elettori, votanti int }
			accMap := map[string]*provAccum{}
			for _, prov := range scraper.Province {
				accMap[prov] = &provAccum{}
			}

			for _, r := range records {
				pp := provMap[r.Provincia]
				acc := accMap[r.Provincia]
				if pp == nil {
					continue
				}
				pp.Comuni++
				acc.elettori += parseItalianInt(r.Elettori)
				if len(r.Rilevamenti) > 0 {
					last := r.Rilevamenti[len(r.Rilevamenti)-1]
					acc.votanti += parseItalianInt(last.Votanti)
				}
			}

			result := RiepilogoRegionale{Anno: anno}
			for _, prov := range scraper.Province {
				pp := provMap[prov]
				if pp == nil {
					continue
				}
				acc := accMap[prov]
				if acc.elettori > 0 {
					perc := float64(acc.votanti) / float64(acc.elettori) * 100
					pp.PercAffluenza = fmt.Sprintf("%.1f", perc)
					pp.TotaleElettori = fmt.Sprintf("%d", acc.elettori)
				}
				result.Province = append(result.Province, *pp)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				data, _ := json.MarshalIndent(result, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Riepilogo Elezioni Comunali Sicilia %d\n\n", anno)
			rows := make([][]string, len(result.Province))
			for i, p := range result.Province {
				perc := "-"
				if p.PercAffluenza != "" {
					perc = p.PercAffluenza + "%"
				}
				rows[i] = []string{p.Provincia, fmt.Sprintf("%d", p.Comuni), perc}
			}
			return flags.printTable(cmd, []string{"PROVINCIA", "COMUNI", "% AFFLUENZA FINALE"}, rows)
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	return cmd
}

// parseItalianInt parses integers formatted with dots as thousand separators (e.g. "12.345" → 12345).
func parseItalianInt(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(s, ".", ""))
	return n
}
