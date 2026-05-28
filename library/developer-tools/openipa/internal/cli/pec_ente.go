// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(WS-endpoint-migration): new command — WS20_PEC, REST-only endpoint (no PHP form).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPecEnteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ente <cod-amm>",
		Short: "PEC attive di un ente per codice IPA (WS20)",
		Long: `Restituisce la lista degli indirizzi PEC attivi di un ente
a partire dal suo codice IPA (COD_AMM).`,
		Example: strings.Trim(`
  openipa-pp-cli pec ente agid
  openipa-pp-cli pec ente agid --json
  openipa-pp-cli pec ente agid --json --select cod_amm,domicilio_digitale`, "\n"),
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

			path := "/ws/WS20PECServices/api/WS20_PEC"
			raw, _, callErr := c.Post(path, map[string]any{"COD_AMM": codAmm})
			if callErr != nil {
				return fmt.Errorf("WS20_PEC: %w", callErr)
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
				fmt.Fprintf(cmd.OutOrStdout(), "Nessuna PEC trovata per il codice IPA: %s\n", codAmm)
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
