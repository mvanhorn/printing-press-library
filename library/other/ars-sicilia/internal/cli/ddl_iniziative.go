package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var ddlIniziative = []string{
	"Consigli comunali",
	"Consigli provinciali",
	"Fatto proprio dalla Commissione",
	"Governativa",
	"Iniziativa Popolare",
	"Parlamentare",
}

func newDdlIniziativeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iniziative",
		Short: "Elenca i tipi di iniziativa dei DDL, da passare a --firmatario in 'ddl cerca'.",
		Long: `Elenca il vocabolario controllato del campo Iniziativa dei DDL.
Questi valori corrispondono alle opzioni del portale ARS per filtrare
i disegni di legge per tipo di proponente.

Non esiste un flag --iniziativa: il portale scrive il tipo di iniziativa
nello stesso campo dei firmatari, quindi il valore va passato a --firmatario.`,
		Example: strings.Trim(`
  ars-sicilia-pp-cli ddl iniziative
  ars-sicilia-pp-cli ddl iniziative --json

  # i ddl di iniziativa governativa (il valore va su --firmatario)
  ars-sicilia-pp-cli ddl cerca --legisl 18 --firmatario Governativa`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), ddlIniziative, flags)
			}
			for _, i := range ddlIniziative {
				fmt.Fprintln(cmd.OutOrStdout(), i)
			}
			return nil
		},
	}
	return cmd
}
