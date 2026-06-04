package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
	"github.com/spf13/cobra"
)

func newComuniCmd(flags *rootFlags) *cobra.Command {
	var anno int
	var provincia string

	cmd := &cobra.Command{
		Use:   "comuni",
		Short: "Elenca i comuni alle elezioni con codice, provincia e nome.",
		Example: strings.Trim(`
  elezioni-sicilia-pp-cli comuni
  elezioni-sicilia-pp-cli comuni --provincia PA --json
  elezioni-sicilia-pp-cli comuni --anno 2024`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[{"nome":"Agrigento","provincia":"AG","codice":"11"}]`)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "would fetch: ReportDatiLista{PROV}.html")
				return nil
			}

			province := scraper.Province
			if provincia != "" {
				province = []string{strings.ToUpper(provincia)}
			}

			var allComuni []scraper.Comune
			for _, prov := range province {
				comuni, err := scraper.FetchComuni(prov, anno)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "avviso: %v\n", err)
					continue
				}
				allComuni = append(allComuni, comuni...)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				data, _ := json.MarshalIndent(allComuni, "", "  ")
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			return flags.printTable(cmd, []string{"PROVINCIA", "CODICE", "NOME"},
				comuniRows(allComuni))
		},
	}

	cmd.Flags().IntVar(&anno, "anno", 2026, "Anno elezioni (2009-2026)")
	cmd.Flags().StringVar(&provincia, "provincia", "", "Filtra per provincia (AG, CL, CT, EN, ME, PA, RG, SR, TP)")
	return cmd
}

func comuniRows(comuni []scraper.Comune) [][]string {
	rows := make([][]string, len(comuni))
	for i, c := range comuni {
		rows[i] = []string{c.Provincia, c.Codice, c.Nome}
	}
	return rows
}
