package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/regionali"
	"github.com/spf13/cobra"
)

const defaultAnnoRegionali = 2022

func newRegionaliCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regionali",
		Short: "Dati elezioni regionali siciliane (Assemblea Regionale Siciliana — ARS).",
		Long: `Comandi per le elezioni regionali (ARS). Anni supportati: 2017, 2022.

Sotto-comandi:
  presidente   Voti dei candidati presidente, lista regionale + liste provinciali collegate
  affluenza    Affluenza per provincia (3 rilevamenti orari) con confronto storico
  seggi        Riparto seggi per lista e per provincia
  listino      Candidati del listino regionale per ogni lista
  candidati    Voti di preferenza ARS per provincia`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(
		newRegionaliPresidenteCmd(flags),
		newRegionaliAffluenzaCmd(flags),
		newRegionaliSeggiCmd(flags),
		newRegionaliListinoCmd(flags),
		newRegionaliCandidatiCmd(flags),
	)
	return cmd
}

func validateAnnoRegionali(anno int) error {
	if !regionali.IsKnownAnno(anno) {
		return fmt.Errorf("anno %d non supportato (valori validi: %v)", anno, regionali.KnownAnni)
	}
	return nil
}

func emitRegionaliJSON(cmd *cobra.Command, flags *rootFlags, data any, meta map[string]any) error {
	out := map[string]any{"meta": meta, "data": data}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal regionali JSON: %w", err)
	}
	if flags.selectFields != "" {
		b = filterFields(b, flags.selectFields)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}

// ---- presidente ----

func newRegionaliPresidenteCmd(flags *rootFlags) *cobra.Command {
	var anno int
	cmd := &cobra.Command{
		Use:   "presidente",
		Short: "Voti dei candidati presidente con lista regionale e liste provinciali collegate.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli regionali presidente
  elezioni-sicilia-pp-cli regionali presidente --anno 2017 --json
  elezioni-sicilia-pp-cli regionali presidente --json --select data.presidenti.nome,data.presidenti.voti`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateAnnoRegionali(anno); err != nil {
				return err
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: rep_7/riepilogoRegionale.html")
				return nil
			}
			r, url, err := regionali.FetchPresidente(anno)
			if err != nil {
				return fmt.Errorf("regionali presidente: %w", err)
			}
			meta := map[string]any{"source": url, "anno": anno, "sezioni": r.Sezioni, "count": len(r.Presidenti)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitRegionaliJSON(cmd, flags, r, meta)
			}
			rows := make([][]string, 0, len(r.Presidenti))
			for _, p := range r.Presidenti {
				rows = append(rows, []string{p.Numero, p.Nome, p.NomeLista, p.Voti, p.Percentuale, p.TotaliSeggi})
			}
			return flags.printTable(cmd, []string{"N°", "PRESIDENTE", "LISTA", "VOTI", "%", "SEGGI"}, rows)
		},
	}
	cmd.Flags().IntVar(&anno, "anno", defaultAnnoRegionali, "Anno elezioni regionali (2017, 2022)")
	return cmd
}

// ---- affluenza ----

func newRegionaliAffluenzaCmd(flags *rootFlags) *cobra.Command {
	var anno int
	cmd := &cobra.Command{
		Use:   "affluenza",
		Short: "Affluenza per provincia con 3 rilevamenti orari e confronto storico.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateAnnoRegionali(anno); err != nil {
				return err
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: rep_6/affluenzaRegionale1..3.html")
				return nil
			}
			r, urls, err := regionali.FetchAffluenza(anno)
			if err != nil {
				return fmt.Errorf("regionali affluenza: %w", err)
			}
			meta := map[string]any{"sources": urls, "anno": anno, "count": len(r.Province)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitRegionaliJSON(cmd, flags, r, meta)
			}
			rows := make([][]string, 0, len(r.Province))
			for _, p := range r.Province {
				last := ""
				if n := len(p.Rilevamenti); n > 0 {
					last = p.Rilevamenti[n-1].Percentuale
				}
				rows = append(rows, []string{p.Provincia, fmt.Sprintf("%d", len(p.Rilevamenti)), last})
			}
			return flags.printTable(cmd, []string{"PROV", "RILEV.", "% FINALE"}, rows)
		},
	}
	cmd.Flags().IntVar(&anno, "anno", defaultAnnoRegionali, "Anno elezioni regionali (2017, 2022)")
	return cmd
}

// ---- seggi ----

func newRegionaliSeggiCmd(flags *rootFlags) *cobra.Command {
	var anno int
	cmd := &cobra.Command{
		Use:   "seggi",
		Short: "Riparto seggi per lista e per provincia.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateAnnoRegionali(anno); err != nil {
				return err
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: rep_8/ripartoSeggi.html")
				return nil
			}
			r, url, err := regionali.FetchSeggi(anno)
			if err != nil {
				return fmt.Errorf("regionali seggi: %w", err)
			}
			meta := map[string]any{"source": url, "anno": anno, "count": len(r.Liste)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitRegionaliJSON(cmd, flags, r, meta)
			}
			rows := make([][]string, 0, len(r.Liste))
			for _, l := range r.Liste {
				rows = append(rows, []string{l.Lista, l.Seggi["TOT"]})
			}
			return flags.printTable(cmd, []string{"LISTA", "SEGGI TOTALI"}, rows)
		},
	}
	cmd.Flags().IntVar(&anno, "anno", defaultAnnoRegionali, "Anno elezioni regionali (2017, 2022)")
	return cmd
}

// ---- listino ----

func newRegionaliListinoCmd(flags *rootFlags) *cobra.Command {
	var anno int
	cmd := &cobra.Command{
		Use:   "listino",
		Short: "Candidati del listino regionale per ogni lista (capolista = candidato presidente).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateAnnoRegionali(anno); err != nil {
				return err
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: rep_9/listeRegionali.html")
				return nil
			}
			r, url, err := regionali.FetchListino(anno)
			if err != nil {
				return fmt.Errorf("regionali listino: %w", err)
			}
			meta := map[string]any{"source": url, "anno": anno, "count": len(r.Liste)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitRegionaliJSON(cmd, flags, r, meta)
			}
			rows := make([][]string, 0)
			for _, l := range r.Liste {
				rows = append(rows, []string{l.Numero, l.Nome, fmt.Sprintf("%d", len(l.Candidati))})
			}
			return flags.printTable(cmd, []string{"N°", "LISTA", "CANDIDATI"}, rows)
		},
	}
	cmd.Flags().IntVar(&anno, "anno", defaultAnnoRegionali, "Anno elezioni regionali (2017, 2022)")
	return cmd
}

// ---- candidati ARS (preference votes per province) ----

func newRegionaliCandidatiCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string
	cmd := &cobra.Command{
		Use:   "candidati",
		Short: "Voti di preferenza dei candidati ARS in una provincia.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli regionali candidati --provincia CT
  elezioni-sicilia-pp-cli regionali candidati --provincia PA --anno 2017 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateAnnoRegionali(anno); err != nil {
				return err
			}
			if provincia == "" {
				return fmt.Errorf("--provincia richiesto (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
			}
			provincia = strings.ToUpper(provincia)
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "would fetch: rep_5/<%s>/votiCandidatiProvincia%s.html\n", regionali.ProvinceCity(provincia), provincia)
				return nil
			}
			r, url, err := regionali.FetchCandidatiARS(anno, provincia)
			if err != nil {
				return fmt.Errorf("regionali candidati: %w", err)
			}
			meta := map[string]any{"source": url, "anno": anno, "provincia": provincia, "count": len(r.Liste)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitRegionaliJSON(cmd, flags, r, meta)
			}
			rows := make([][]string, 0)
			for _, l := range r.Liste {
				rows = append(rows, []string{l.Numero, l.Nome, fmt.Sprintf("%d", len(l.Candidati))})
			}
			return flags.printTable(cmd, []string{"N°", "LISTA", "CANDIDATI"}, rows)
		},
	}
	cmd.Flags().IntVar(&anno, "anno", defaultAnnoRegionali, "Anno elezioni regionali (2017, 2022)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP) — obbligatoria")
	return cmd
}
