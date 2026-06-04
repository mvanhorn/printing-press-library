// pp:data-source live
// pp:client-call
// Novel feature — profilo deputato cross-archive: aggrega tutti gli atti
// firmati (FIRMAT) o pronunciati (ORATOR) da un parlamentare.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelDeputatoProfiloCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl int
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:     "profilo <nome>",
		Short:   "Aggrega in un'unica vista tutti gli atti firmati o pronunciati da un deputato.",
		Example: "  ars-sicilia-pp-cli deputato profilo \"Rossi Mario\" --legisl 18 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(strings.Join(args, " "))
			if name == "" {
				return fmt.Errorf("nome del deputato richiesto (es. \"Rossi Mario\")")
			}
			return runDeputatoProfilo(cmd, flags, name, flagLegisl, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18). 0 = tutte le legislature.")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max risultati per archivio.")
	return cmd
}

type profileItem struct {
	Tipo      string `json:"tipo"`
	Archivio  string `json:"archivio"`
	DocID     int    `json:"doc_id"`
	Numero    string `json:"numero,omitempty"`
	Data      string `json:"data,omitempty"`
	Titolo    string `json:"titolo"`
	Firmatari string `json:"firmatari,omitempty"`
	URL       string `json:"url,omitempty"`
}

type profileReport struct {
	Deputato  string         `json:"deputato"`
	Legisl    int            `json:"legisl,omitempty"`
	Conteggio map[string]int `json:"conteggio"`
	Atti      []profileItem  `json:"atti"`
}

func runDeputatoProfilo(cmd *cobra.Command, flags *rootFlags, name string, legisl, perArchive int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if perArchive <= 0 {
		perArchive = 30
	}
	report := profileReport{
		Deputato:  name,
		Legisl:    legisl,
		Conteggio: map[string]int{},
	}

	// Archivi con FIRMAT.
	firmaArchives := []string{"ddl", "interrogazioni", "interpellanze", "mozioni", "odg", "risoluzioni"}
	for _, slug := range firmaArchives {
		arc := icaro.BySlug(slug)
		if arc == nil {
			continue
		}
		c, err := icaro.New(nil)
		if err != nil {
			continue
		}
		params := map[string]string{"firmatario": name}
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:   params,
			Limit:    perArchive,
			MaxPages: maxInt(1, (perArchive+9)/10),
		})
		if err != nil {
			continue
		}
		for _, r := range recs {
			report.Atti = append(report.Atti, profileItem{
				Tipo:      slug,
				Archivio:  arc.ID,
				DocID:     r.DocID,
				Numero:    r.Fields["Numero"],
				Data:      r.Fields["Data"],
				Titolo:    r.Title,
				Firmatari: r.Fields["Firmatari"],
				URL:       r.URL,
			})
			report.Conteggio[slug]++
		}
	}

	// Resoconti d'aula con free-text match sul nome dell'oratore.
	if arc := icaro.BySlug("resoconti"); arc != nil {
		c, err := icaro.New(nil)
		if err != nil {
			return nil
		}
		params := map[string]string{"testo": name}
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:   params,
			Limit:    perArchive,
			MaxPages: maxInt(1, (perArchive+9)/10),
		})
		if err == nil {
			for _, r := range recs {
				report.Atti = append(report.Atti, profileItem{
					Tipo:     "resoconti",
					Archivio: arc.ID,
					DocID:    r.DocID,
					Numero:   r.Fields["Numero"],
					Data:     r.Fields["Data"],
					Titolo:   r.Title,
					URL:      r.URL,
				})
				report.Conteggio["resoconti"]++
			}
		}
	}

	// Sort by date (reverse chronological).
	sort.SliceStable(report.Atti, func(i, j int) bool {
		return parseICaroDate(report.Atti[i].Data) > parseICaroDate(report.Atti[j].Data)
	})

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(out, "Deputato: %s\n", report.Deputato)
	if report.Legisl > 0 {
		fmt.Fprintf(out, "Legislatura: %d\n", report.Legisl)
	}
	fmt.Fprintf(out, "\nConteggi per archivio:\n")
	for k, v := range report.Conteggio {
		fmt.Fprintf(out, "  %-15s %d\n", k, v)
	}
	fmt.Fprintf(out, "\nAtti (%d totali):\n", len(report.Atti))
	for _, a := range report.Atti {
		fmt.Fprintf(out, "  [%s] %s  %s\n      %s\n", a.Tipo, a.Data, a.Numero, a.Titolo)
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
