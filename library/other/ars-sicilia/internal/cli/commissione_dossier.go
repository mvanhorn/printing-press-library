// pp:data-source live
// pp:client-call
// Novel feature — dossier 360° di una commissione: convocazioni, sommari,
// pareri al governo e DDL assegnati raggruppati per codice commissione.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelCommissioneDossierCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl int
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:     "dossier <codcom-o-nome>",
		Short:   "Vista completa di una commissione: convocazioni, sommari, pareri al Governo e DDL assegnati.",
		Example: "  ars-sicilia-pp-cli commissione dossier 5 --legisl 18 --json",
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
			arg := strings.TrimSpace(strings.Join(args, " "))
			return runCommissioneDossier(cmd, flags, arg, flagLegisl, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18).")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max risultati per sezione.")
	return cmd
}

type dossierSection struct {
	Tipo      string           `json:"tipo"`
	Archivio  string           `json:"archivio"`
	Risultati []map[string]any `json:"risultati"`
}

type dossierReport struct {
	Commissione string           `json:"commissione"`
	Legisl      int              `json:"legisl,omitempty"`
	Conteggio   map[string]int   `json:"conteggio"`
	Sezioni     []dossierSection `json:"sezioni"`
}

func runCommissioneDossier(cmd *cobra.Command, flags *rootFlags, arg string, legisl, perSection int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if perSection <= 0 {
		perSection = 30
	}
	report := dossierReport{
		Commissione: arg,
		Legisl:      legisl,
		Conteggio:   map[string]int{},
	}

	// Detect whether arg is a numeric codice commissione or a name string.
	codeKey, nameKey := "codcom", "commissione"
	isNumeric := true
	for _, r := range arg {
		if r < '0' || r > '9' {
			isNumeric = false
			break
		}
	}

	section := func(slug, label, paramKey string) {
		arc := icaro.BySlug(slug)
		if arc == nil {
			return
		}
		c, err := icaro.New(nil)
		if err != nil {
			return
		}
		params := map[string]string{paramKey: arg}
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:   params,
			Limit:    perSection,
			MaxPages: maxInt(1, (perSection+9)/10),
		})
		if err != nil {
			return
		}
		s := dossierSection{Tipo: label, Archivio: arc.ID}
		for _, r := range recs {
			s.Risultati = append(s.Risultati, map[string]any{
				"doc_id":  r.DocID,
				"data":    r.Fields["Data"],
				"numero":  r.Fields["Numero"],
				"titolo":  r.Title,
				"excerpt": r.Excerpt,
				"url":     r.URL,
			})
		}
		report.Sezioni = append(report.Sezioni, s)
		report.Conteggio[label] = len(s.Risultati)
	}

	pickKey := nameKey
	if isNumeric {
		pickKey = codeKey
	}
	section("convocazioni", "convocazioni", pickKey)
	section("sommari", "sommari", pickKey)
	section("pareri", "pareri", "commissione")
	if !isNumeric {
		// Filter DDL by relatore/commissione name as free-text (no commission
		// field in DDL archive); use the testo path.
		section("ddl", "ddl_assegnati", "testo")
	}

	total := 0
	for _, v := range report.Conteggio {
		total += v
	}
	if total == 0 {
		return fmt.Errorf("nessuna commissione trovata per: %q", arg)
	}

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(out, "Commissione: %s\n", report.Commissione)
	if report.Legisl > 0 {
		fmt.Fprintf(out, "Legislatura: %d\n\n", report.Legisl)
	}
	for _, s := range report.Sezioni {
		fmt.Fprintf(out, "[%s] %d risultati\n", s.Tipo, len(s.Risultati))
		for _, r := range s.Risultati {
			fmt.Fprintf(out, "  #%v  %v  %v\n", r["doc_id"], r["data"], r["titolo"])
		}
		fmt.Fprintln(out)
	}
	return nil
}
