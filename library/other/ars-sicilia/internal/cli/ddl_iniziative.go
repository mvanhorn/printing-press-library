package cli

import (
	"encoding/json"
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
		Short: "Elenca i tipi di iniziativa disponibili per il campo Iniziativa in 'ddl cerca'.",
		Long: `Elenca il vocabolario controllato del campo Iniziativa dei DDL.
Questi valori corrispondono alle opzioni del portale ARS per filtrare
i disegni di legge per tipo di proponente.`,
		Example: strings.Trim(`
  ars-sicilia-pp-cli ddl iniziative
  ars-sicilia-pp-cli ddl iniziative --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(ddlIniziative)
			}
			for _, i := range ddlIniziative {
				fmt.Fprintln(cmd.OutOrStdout(), i)
			}
			return nil
		},
	}
	return cmd
}
