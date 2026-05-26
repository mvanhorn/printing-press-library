// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: servizi tipi command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newServiziTipiCmd(flags *rootFlags) *cobra.Command {
	var perUo bool

	cmd := &cobra.Command{
		Use:   "tipi",
		Short: "Lista tipologie servizi digitali (ente) o categorie servizi (UO)",
		Long: `Lista gli ID da usare nelle ricerche dei servizi digitali.

Senza flag mostra le tipologie dei servizi degli enti, da usare con:
  servizi ente --tipologia <id>

Con --uo mostra le categorie dei servizi delle unità organizzative, da usare con:
  servizi uo --categoria <id>

Esempio importante: per gli enti, id=1 è "Accesso agli atti" e comprende
spesso l'Albo Pretorio online.`,
		Example: `  # Tipologie servizi ente
  openipa-pp-cli servizi tipi

  # Trova l'ID per Accesso agli atti / Albo Pretorio
  openipa-pp-cli servizi tipi --json | jq '.data[] | select(.tipo == "Accesso agli atti")'

  # Categorie servizi UO
  openipa-pp-cli servizi tipi --uo

  # Output compatto con id e tipo
  openipa-pp-cli servizi tipi --json | jq '.data[] | {id, tipo}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newPortaleClient(flags)

			path := "/api/tiposerviziodigitale"
			resource := "servizi/tipi-ente"
			if perUo {
				path = "/api/serviziufficio/tipoServiziUfficio"
				resource = "servizi/tipi-uo"
			}

			raw, status, err := c.GetJSON(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// risposta: {"errore":false,"risposta":[...],...}
			var wrapper struct {
				Errore            bool            `json:"errore"`
				Risposta          json.RawMessage `json:"risposta"`
				DescrizioneErrore *string         `json:"descrizioneErrore"`
			}
			if err := json.Unmarshal(raw, &wrapper); err != nil {
				return fmt.Errorf("parsing risposta tipi: %w", err)
			}
			if wrapper.Errore {
				msg := "portale error"
				if wrapper.DescrizioneErrore != nil && *wrapper.DescrizioneErrore != "" {
					msg = *wrapper.DescrizioneErrore
				}
				return fmt.Errorf("%s: %s", path, msg)
			}

			items := wrapper.Risposta
			if items == nil {
				items = json.RawMessage(`[]`)
			}

			filtered := items
			if flags.selectFields != "" {
				filtered = filterFields(filtered, flags.selectFields)
			}

			w := cmd.OutOrStdout()
			if wantsHumanTable(w, flags) && flags.selectFields == "" {
				var list []map[string]any
				if err := json.Unmarshal(items, &list); err == nil && len(list) > 0 {
					if err2 := printAutoTable(w, list); err2 == nil {
						return nil
					}
				}
			}

			var parsed any
			envelope := map[string]any{
				"action":   "get",
				"resource": resource,
				"path":     path,
				"status":   status,
				"success":  status >= 200 && status < 300,
			}
			if err := json.Unmarshal(filtered, &parsed); err == nil {
				envelope["data"] = parsed
			}
			envelopeJSON, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			return printOutput(w, json.RawMessage(envelopeJSON), true)
		},
	}

	cmd.Flags().BoolVar(&perUo, "uo", false, "Lista categorie servizi per UO invece delle tipologie servizio ente")
	return cmd
}
