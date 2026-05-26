// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// PATCH(WS-endpoint-migration): new command — WS19_AOO, REST-only endpoint (no PHP form).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newAooStoricoCmd(flags *rootFlags) *cobra.Command {
	var codUniAOO string
	var storico bool

	cmd := &cobra.Command{
		Use:   "storico <cod-amm>",
		Short: "Lista AOO di un ente (attive e cessate) per codice IPA (WS19)",
		Long: `Restituisce la lista delle Aree Organizzative Omogenee di un ente,
comprese quelle cessate. Opzionalmente filtra per singola AOO tramite codice univoco.`,
		Example: strings.Trim(`
  openipa-pp-cli aoo storico agid
  openipa-pp-cli aoo storico agid --storico
  openipa-pp-cli aoo storico agid --aoo A463BFE --json`, "\n"),
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

			body := map[string]any{"COD_AMM": codAmm}
			if codUniAOO != "" {
				body["COD_UNI_AOO"] = codUniAOO
			}
			if storico {
				body["STORICO"] = "S"
			}

			path := "/ws/WS19AOOServices/api/WS19_AOO"
			raw, _, callErr := c.Post(path, body)
			if callErr != nil {
				return fmt.Errorf("WS19_AOO: %w", callErr)
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
				fmt.Fprintf(cmd.OutOrStdout(), "Nessuna AOO trovata per il codice IPA: %s\n", codAmm)
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
	cmd.Flags().StringVar(&codUniAOO, "aoo", "", "Codice univoco AOO (opzionale, es. A463BFE — usa 'aoo storico <cod_amm> --json | jq .[].cod_uni_aoo' per trovarlo)")
	cmd.Flags().BoolVar(&storico, "storico", false, "Includi AOO cessate nei risultati (default: solo AOO attive)")
	return cmd
}
