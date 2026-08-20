// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: sede uo command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSedeUoCmd(flags *rootFlags) *cobra.Command {
	var nome, cf, area, categoria, codiceEnte, codiceUo string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "uo",
		Short: "Cerca UO per sede: nome, area geografica, ente",
		Example: `  openipa-pp-cli sede uo --nome "ragioneria"
  openipa-pp-cli sede uo --codice-ente c_g273
  openipa-pp-cli sede uo --area "Milano" --tutti`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nome == "" && cf == "" && area == "" && categoria == "" && codiceEnte == "" && codiceUo == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --nome, --cf, --area, --categoria, --codice-ente, --codice-uo")
			}
			c := newPortaleClient(flags)

			body := map[string]any{
				"descrizione":       nilIfEmpty(nome),
				"codiceFiscale":     nilIfEmpty(cf),
				"area":              nilIfEmpty(area),
				"codiceCategoria":   nilIfEmpty(categoria),
				"denominazioneEnte": nil,
				"codEnte":           nilIfEmpty(codiceEnte),
				"codUniOu":          nilIfEmpty(codiceUo),
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/ou", body, "codUniOu")
			} else {
				body["paginazione"] = newPaginazione("codUniOu", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/ou", body)
				if err == nil {
					var r *portaleResp
					r, err = parsePortaleResp(raw)
					if err == nil {
						if r.Risposta == nil {
							items = json.RawMessage(`[]`)
						} else {
							items, pag = r.Risposta.ListaResponse, r.Risposta.Paginazione
						}
					}
				}
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/ou", "sede/uo", status, items, pag)
		},
	}

	cmd.Flags().StringVar(&nome, "nome", "", "Parte del nome dell'UO")
	cmd.Flags().StringVar(&cf, "cf", "", "Codice fiscale dell'ente")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica (es. 'Milano', 'Roma') — senza sigla provinciale")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().StringVar(&codiceEnte, "codice-ente", "", "Codice IPA dell'ente")
	cmd.Flags().StringVar(&codiceUo, "codice-uo", "", "Codice univoco UO")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine e restituisce risultati completi")
	return cmd
}
