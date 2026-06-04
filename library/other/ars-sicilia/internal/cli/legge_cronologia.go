// pp:data-source live
// pp:client-call
// Novel feature — cronologia inversa di una legge regionale: dalla legge
// promulgata risale al DDL originario e ai passaggi parlamentari.

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

func newNovelLeggeCronologiaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cronologia <legisl> <numero>",
		Short: "Inversa di `ddl iter`: dalla legge promulgata risale al DDL originario e ai passaggi parlamentari.",
		Long: `Usare questo comando solo per una legge GIA' promulgata (archivio 201).
Per un DDL ancora in iter usare ` + "`ars-sicilia ddl iter`" + `.`,
		Example: "  ars-sicilia-pp-cli legge cronologia 18 5 --json",
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
			return runLeggeCronologia(cmd, flags, legisl, numero)
		},
	}
	return cmd
}

func runLeggeCronologia(cmd *cobra.Command, flags *rootFlags, legisl, numero int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	report := iterReport{Legisl: legisl, Numero: numero}

	// 1. La legge (archivio 201).
	c, err := icaro.New(nil)
	if err != nil {
		return fmt.Errorf("creazione client icaro: %w", err)
	}
	if arc := icaro.BySlug("leggi"); arc != nil {
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params: map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)},
			Limit:  3,
		})
		if err == nil {
			for _, r := range recs {
				if report.Titolo == "" {
					report.Titolo = r.Title
				}
				report.Eventi = append(report.Eventi, iterEvent{
					Fase:      "promulgazione",
					Data:      r.Fields["Data"],
					Sede:      "Legge regionale",
					Titolo:    r.Title,
					URL:       r.URL,
					ArchiveID: arc.ID,
					DocID:     r.DocID,
				})
			}
		}
	}

	// 2. Risali al DDL originario via free-text sul titolo della legge.
	if report.Titolo != "" {
		if arc := icaro.BySlug("ddl"); arc != nil {
			c2, _ := icaro.New(nil)
			// Usa solo le prime 4 parole significative come query per
			// evitare match troppo lunghi.
			words := strings.Fields(report.Titolo)
			if len(words) > 4 {
				words = words[:4]
			}
			query := strings.Join(words, " ")
			recs, err := c2.Search(ctx, *arc, icaro.SearchOptions{
				Params: map[string]string{
					"legisl": itoa(legisl),
					"testo":  query,
				},
				Limit:    5,
				MaxPages: 1,
			})
			if err == nil {
				for _, r := range recs {
					report.Eventi = append(report.Eventi, iterEvent{
						Fase:      "ddl_originario",
						Data:      r.Fields["Data"],
						Sede:      "Disegno di legge n. " + r.Fields["Numero"],
						Titolo:    r.Title,
						URL:       r.URL,
						ArchiveID: arc.ID,
						DocID:     r.DocID,
					})
				}
			}
		}
	}

	// 3. Sommari di commissione che citano la legge nel testo.
	if arc := icaro.BySlug("sommari"); arc != nil {
		c3, _ := icaro.New(nil)
		recs, err := c3.Search(ctx, *arc, icaro.SearchOptions{
			Params: map[string]string{
				"legisl": itoa(legisl),
				"testo":  fmt.Sprintf("legge %d", numero),
			},
			Limit:    10,
			MaxPages: 1,
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

	sort.SliceStable(report.Eventi, func(i, j int) bool {
		return parseICaroDate(report.Eventi[i].Data) < parseICaroDate(report.Eventi[j].Data)
	})

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(out, "Legge %d/%d — %s\n", report.Legisl, report.Numero, report.Titolo)
	for _, e := range report.Eventi {
		fmt.Fprintf(out, "  [%s] %s — %s\n", e.Fase, e.Data, strings.TrimSpace(e.Sede+" "+e.Titolo))
	}
	return nil
}
