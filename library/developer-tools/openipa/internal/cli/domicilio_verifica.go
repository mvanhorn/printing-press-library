// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type domicilioStatus struct {
	PEC     string           `json:"pec"`
	Status  string           `json:"status"` // attivo | storico | sconosciuto
	Tipo    string           `json:"tipo,omitempty"`
	DataPub string           `json:"data_pubblicazione,omitempty"`
	DataCan string           `json:"data_cancellazione,omitempty"`
	Entita  []map[string]any `json:"entita,omitempty"`
	Note    string           `json:"note,omitempty"`
}

func newDomicilioVerificaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verifica <indirizzo-pec>",
		Short: "Verifica se una PEC è domicilio digitale attivo, storico o sconosciuta",
		Long: `Controlla lo stato di un indirizzo PEC nell'anagrafica IPA:
  - WS13_DOM_DIG: cerca per domicilio digitale (attivo)
  - WS07_EMAIL: cerca l'entità IPA associata all'email

Produce una classificazione: attivo / storico / sconosciuto.
Utile prima di inviare comunicazioni ufficiali per verificare che la PEC sia valida.`,
		Example: strings.Trim(`
  openipa-pp-cli domicilio verifica ente@pec.esempio.it
  openipa-pp-cli domicilio verifica ente@pec.esempio.it --json`, "\n"),
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

			result := domicilioStatus{PEC: pec, Status: "sconosciuto"}

			// WS13_DOM_DIG: cerca per domicilio digitale (PEC attiva)
			// Note: the param name has a space: "DOMICILIO DIGITALE"
			raw13, _, err13 := c.Post("/ws/WS13DOMDIGServices/api/WS13_DOM_DIG", map[string]any{"DOMICILIO DIGITALE": pec})
			if err13 == nil {
				items := ipaExtractItems(raw13)
				if len(items) > 0 {
					result.Status = "attivo"
					result.Entita = items
					if v, ok := items[0]["tipo"].(string); ok {
						result.Tipo = v
					}
					if v, ok := items[0]["data_pubblicazione"].(string); ok {
						result.DataPub = v
					}
					// Check if it also has data_cancellazione (means storico)
					if v, ok := items[0]["data_cancellazione"].(string); ok && v != "" && v != "null" {
						result.Status = "storico"
						result.DataCan = v
					}
				}
			}

			// WS07_EMAIL: cerca anche per email generica
			if result.Status == "sconosciuto" {
				raw07, _, err07 := c.Post("/ws/WS07EMAILServices/api/WS07_EMAIL", map[string]any{"EMAIL": pec})
				if err07 == nil {
					items := ipaExtractItems(raw07)
					if len(items) > 0 {
						result.Entita = items
						result.Note = "trovata come email registrata (non come domicilio digitale)"
					}
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Human output
			w := cmd.OutOrStdout()
			statusIcon := "✗"
			if result.Status == "attivo" {
				statusIcon = "✓"
			} else if result.Status == "storico" {
				statusIcon = "⚠"
			}
			fmt.Fprintf(w, "%s PEC: %s — %s\n", statusIcon, pec, strings.ToUpper(result.Status))
			if result.Tipo != "" {
				fmt.Fprintf(w, "   Tipo: %s\n", result.Tipo)
			}
			if result.DataPub != "" {
				fmt.Fprintf(w, "   Pubblicata: %s\n", result.DataPub)
			}
			if result.DataCan != "" {
				fmt.Fprintf(w, "   Cancellata: %s\n", result.DataCan)
			}
			for _, e := range result.Entita {
				des, _ := e["des_amm"].(string)
				cod, _ := e["cod_amm"].(string)
				tipo, _ := e["tipo_entita"].(string)
				if des != "" {
					fmt.Fprintf(w, "   Ente: %s (%s) [%s]\n", des, cod, tipo)
				}
			}
			if result.Note != "" {
				fmt.Fprintf(w, "   Nota: %s\n", result.Note)
			}
			return nil
		},
	}
	return cmd
}
