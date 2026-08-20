// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(WS-endpoint-migration): new command — WS22_PEC_STOR, REST-only endpoint (no PHP form).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPecCercaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cerca <indirizzo-pec>",
		Short: "Storia di un indirizzo PEC nell'IPA (WS22)",
		Long: `Recupera la storia completa di un indirizzo PEC nell'Indice PA:
periodi in cui è stato domicilio digitale, ente titolare, date di attivazione/cessazione.`,
		Example: strings.Trim(`
  openipa-pp-cli pec cerca protocollo@pec.agid.gov.it
  openipa-pp-cli pec cerca protocollo@pec.agid.gov.it --json`, "\n"),
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

			pec := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ws/WS22PECSTORServices/api/WS22_PEC_STOR"
			raw, _, callErr := c.Post(path, map[string]any{"PEC": pec})
			if callErr != nil {
				return fmt.Errorf("WS22_PEC_STOR: %w", callErr)
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
				fmt.Fprintf(cmd.OutOrStdout(), "Nessuna storia trovata per l'indirizzo PEC: %s\n", pec)
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
