// pp:data-source live
// pp:client-call
// Novel feature — ricostruisce la cronologia completa di un DDL.
// Combina ricerche su più archivi (DDL 221, sommari commissione 230,
// resoconti d'aula 217) usando direttamente l'icaroclient.

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

func newNovelDdlIterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "iter <legisl> <numero>",
		Short:   "Ricostruisce la cronologia completa di un disegno di legge: presentazione, commissione, aula, eventuale legge.",
		Example: "  ars-sicilia-pp-cli ddl iter 18 1500 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			if dryRunOK(flags) {
				return nil
			}
			legisl, err := atoiArg(args[0], "legisl")
			if err != nil {
				return err
			}
			numero, err := atoiArg(args[1], "numero")
			if err != nil {
				return err
			}
			return runDdlIter(cmd, flags, legisl, numero)
		},
	}
	return cmd
}

type iterEvent struct {
	Fase      string `json:"fase"`
	Data      string `json:"data,omitempty"`
	Sede      string `json:"sede,omitempty"`
	Titolo    string `json:"titolo,omitempty"`
	Oratori   string `json:"oratori,omitempty"`
	URL       string `json:"url,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
	DocID     int    `json:"doc_id,omitempty"`
}

type iterReport struct {
	Legisl int         `json:"legisl"`
	Numero int         `json:"numero"`
	Titolo string      `json:"titolo,omitempty"`
	Eventi []iterEvent `json:"eventi"`
	Note   string      `json:"note,omitempty"`
}

func runDdlIter(cmd *cobra.Command, flags *rootFlags, legisl, numero int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	report := iterReport{Legisl: legisl, Numero: numero, Eventi: []iterEvent{}}

	// 1. DDL stesso (archivio 221).
	if arc := icaro.BySlug("ddl"); arc != nil {
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params: map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)},
			Limit:  1,
		})
		if err == nil && len(recs) > 0 {
			report.Titolo = recs[0].Title
			report.Eventi = append(report.Eventi, iterEvent{
				Fase:      "presentazione",
				Data:      recs[0].Fields["Data"],
				Sede:      "Assemblea (presentazione DDL)",
				Titolo:    recs[0].Title,
				URL:       recs[0].URL,
				ArchiveID: arc.ID,
				DocID:     recs[0].DocID,
			})
		} else {
			report.Note = fmt.Sprintf("DDL %d non trovato nell'archivio della legislatura %d. Verifica legisl e numero con `ars-sicilia-pp-cli ddl cerca`.", numero, legisl)
		}
	}

	// 2. Sommari di commissione che citano il DDL nel testo (archivio 230).
	if arc := icaro.BySlug("sommari"); arc != nil {
		c2, err := icaro.New(nil)
		if err == nil {
			recs, err := c2.Search(ctx, *arc, icaro.SearchOptions{
				Params: map[string]string{
					"legisl": itoa(legisl),
					"testo":  fmt.Sprintf("ddl %d", numero),
				},
				Limit:    20,
				MaxPages: 2,
			})
			if err == nil {
				for _, r := range recs {
					report.Eventi = append(report.Eventi, iterEvent{
						Fase:      "commissione",
						Data:      r.Fields["Data"],
						Sede:      r.Fields["Commissione"],
						Titolo:    r.Title,
						URL:       r.URL,
						ArchiveID: arc.ID,
						DocID:     r.DocID,
					})
				}
			}
		}
	}

	// 3. Resoconti d'aula che citano il DDL (archivio 217).
	if arc := icaro.BySlug("resoconti"); arc != nil {
		c3, err := icaro.New(nil)
		if err == nil {
			recs, err := c3.Search(ctx, *arc, icaro.SearchOptions{
				Params: map[string]string{
					"legisl": itoa(legisl),
					"testo":  fmt.Sprintf("ddl %d", numero),
				},
				Limit:    10,
				MaxPages: 1,
			})
			if err == nil {
				for _, r := range recs {
					report.Eventi = append(report.Eventi, iterEvent{
						Fase:      "aula",
						Data:      r.Fields["Data"],
						Sede:      "Assemblea (aula)",
						Titolo:    r.Title,
						Oratori:   r.Fields["Oratori"],
						URL:       r.URL,
						ArchiveID: arc.ID,
						DocID:     r.DocID,
					})
				}
			}
		}
	}

	// Sort events by ISO date when parseable.
	sort.SliceStable(report.Eventi, func(i, j int) bool {
		return parseICaroDate(report.Eventi[i].Data) < parseICaroDate(report.Eventi[j].Data)
	})

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(out, "DDL %d/%d — %s\n", report.Legisl, report.Numero, report.Titolo)
	for _, e := range report.Eventi {
		fmt.Fprintf(out, "  [%s] %s — %s\n", e.Fase, e.Data, strings.TrimSpace(e.Sede+" "+e.Titolo))
	}
	return nil
}

// parseICaroDate converts DD.MM.YYYY (or DD.MM.YY) into a sortable string
// "YYYY-MM-DD"; returns the input as-is when the format isn't recognized.
func parseICaroDate(s string) string {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 3 {
		return s
	}
	dd, mm, yy := parts[0], parts[1], parts[2]
	if len(yy) == 2 {
		// Crude century pivot — atti precedenti al 2000 sono rari nel sito.
		if yy[0] >= '0' && yy[0] <= '4' {
			yy = "20" + yy
		} else {
			yy = "19" + yy
		}
	}
	if len(yy) != 4 || len(mm) > 2 || len(dd) > 2 {
		return s
	}
	if len(mm) == 1 {
		mm = "0" + mm
	}
	if len(dd) == 1 {
		dd = "0" + dd
	}
	return yy + "-" + mm + "-" + dd
}
