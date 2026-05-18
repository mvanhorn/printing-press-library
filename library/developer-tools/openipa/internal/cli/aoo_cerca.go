// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newAooCercaCmd searches for AOO by unique code (WS18).
// Note: WS18_AOO takes COD_UNI_AOO (unique AOO code, e.g. "agid_aoo"),
// not a text description — search by name is only available via offline FTS5 after sync.
func newAooCercaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cerca <cod-uni-aoo>",
		Short: "Dati di una AOO per codice univoco AOO (WS18)",
		Long: `Chiama WS18_AOO con il codice univoco di una Area Organizzativa Omogenea
e restituisce i dati completi della AOO.

Il codice univoco AOO ha tipicamente il formato '<cod_amm>_<cod_aoo>',
es. 'agid_aoo' per la AOO principale di AGID.

Per cercare AOO per nome, esegui prima 'openipa sync' e poi usa
la ricerca offline: 'openipa search aoo <nome>'.`,
		Example: strings.Trim(`
  openipa-pp-cli aoo cerca agid_aoo
  openipa-pp-cli aoo cerca agid_aoo --json
  openipa-pp-cli aoo cerca agid_aoo --json --select cod_amm,cod_aoo,des_aoo,mail1`, "\n"),
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

			codUniAOO := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			raw, _, callErr := c.Post("/WS18_AOO.php", map[string]any{"COD_UNI_AOO": codUniAOO})
			if callErr != nil {
				return fmt.Errorf("WS18_AOO: %w", callErr)
			}

			// Detect server-side validation errors (cod_err 70-72) and surface as error
			var apiResp struct {
				Result struct {
					CodErr  int    `json:"cod_err"`
					DescErr string `json:"desc_err"`
				} `json:"result"`
			}
			if json.Unmarshal(raw, &apiResp) == nil && apiResp.Result.CodErr >= 70 && apiResp.Result.CodErr <= 72 {
				return fmt.Errorf("COD_UNI_AOO non valido: %s (il formato atteso è un codice alfanumerico IPA, es. 'agid_aoo')", codUniAOO)
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
				fmt.Fprintf(cmd.OutOrStdout(), "Nessuna AOO trovata per codice univoco: %s\n", codUniAOO)
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
