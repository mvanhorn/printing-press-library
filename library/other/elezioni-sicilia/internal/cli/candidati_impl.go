package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newCandidatiCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "candidati [comune]",
		Short: "Voti per candidato sindaco in un comune.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli candidati Agrigento
  elezioni-sicilia-pp-cli candidati Palermo --json
  elezioni-sicilia-pp-cli candidati Catania --anno 2024
  elezioni-sicilia-pp-cli candidati Messina --provincia ME --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"comune":"Agrigento","provincia":"AG","anno":2026,"stato_scrutinio":"completo","candidati":[]}`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "would fetch: ReportCandidati{PROV}{CODE}.html for %s\n", args[0])
				return nil
			}

			comune, err := scraper.ResolveComune(args[0], provincia, anno)
			if err != nil {
				return fmt.Errorf("candidati: %w", err)
			}

			result, srcURL, err := scraper.FetchCandidati(comune, anno)
			if err != nil {
				return fmt.Errorf("candidati: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				out := map[string]any{
					"meta": map[string]any{"source": srcURL},
					"data": result,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Human table
			fmt.Fprintf(cmd.OutOrStdout(), "Elezioni Comunali %d — %s (%s)\n", anno, result.Comune, result.Provincia)
			fmt.Fprintf(cmd.OutOrStdout(), "Stato scrutinio: %s%s\n\n", result.Stato,
				func() string {
					if result.Dettaglio != "" {
						return " (" + result.Dettaglio + ")"
					}
					return ""
				}())

			return flags.printTable(cmd, []string{"N°", "CANDIDATO", "VOTI", "%"},
				candidatiRows(result))
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}

func candidatiRows(result *scraper.RiportoCandidati) [][]string {
	rows := make([][]string, len(result.Candidati))
	for i, c := range result.Candidati {
		rows[i] = []string{c.Numero, c.Nome, c.Voti, c.Percento}
	}
	return rows
}
