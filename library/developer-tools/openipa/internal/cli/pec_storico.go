// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// PATCH(WS-endpoint-migration): new command — WS21_PEC_ENTE_STOR, REST-only endpoint (no PHP form).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPecStoricoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storico <cod-amm>",
		Short: "Storico PEC di un ente per codice IPA (WS21)",
		Long: `Restituisce lo storico completo degli indirizzi PEC di un ente
(attivi e cessati) a partire dal suo codice IPA (COD_AMM).`,
		Example: strings.Trim(`
  openipa-pp-cli pec storico agid
  openipa-pp-cli pec storico agid --json`, "\n"),
		Args: cobra.MaximumNArgs(1),
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

			codAmm := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ws/WS21PECENTESTORServices/api/WS21_PEC_ENTE_STOR"
			raw, _, callErr := c.Post(path, map[string]any{"COD_AMM": codAmm})
			if callErr != nil {
				return fmt.Errorf("WS21_PEC_ENTE_STOR: %w", callErr)
			}

			items := ipaExtractItems(raw)
			if items == nil {
				items = []map[string]any{}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if len(items) == 1 {
					return enc.Encode(items[0])
				}
				return enc.Encode(items)
			}

			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Nessuno storico PEC trovato per il codice IPA: %s\n", codAmm)
				return nil
			}

			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}
			return nil
		},
	}
	return cmd
}
