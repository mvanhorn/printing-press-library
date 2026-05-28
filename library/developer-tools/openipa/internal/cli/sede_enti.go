// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: sede enti command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSedeEntiCmd(flags *rootFlags) *cobra.Command {
	var nome, cf, area, categoria, codice string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "enti",
		Short: "Cerca enti per sede: nome, CF, area geografica, categoria",
		Example: `  openipa-pp-cli sede enti --nome "Comune di Roma"
  openipa-pp-cli sede enti --cf 80016350821
  openipa-pp-cli sede enti --area "Roma" --categoria "Comuni"
  openipa-pp-cli sede enti --nome "prefettura" --tutti`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nome == "" && cf == "" && area == "" && categoria == "" && codice == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --nome, --cf, --area, --categoria, --codice")
			}
			c := newPortaleClient(flags)

			body := map[string]any{
				"denominazione":          nilIfEmpty(nome),
				"codiceFiscaleRicerca":   nilIfEmpty(cf),
				"area":                   nilIfEmpty(area),
				"codEnte":                nilIfEmpty(codice),
				"codiceCategoria":        nilIfEmpty(categoria),
				"lingueMinoritarie":      nil,
				"idTipoServizioDigitale": nil,
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/ente/ricerca", body, "idEnte")
			} else {
				body["paginazione"] = newPaginazione("idEnte", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/ente/ricerca", body)
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
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/ente/ricerca", "sede/enti", status, items, pag)
		},
	}

	cmd.Flags().StringVar(&nome, "nome", "", "Parte del nome dell'ente")
	cmd.Flags().StringVar(&cf, "cf", "", "Codice fiscale dell'ente")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica (es. 'Milano', 'Roma') — senza sigla provinciale")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().StringVar(&codice, "codice", "", "Codice IPA dell'ente")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine e restituisce risultati completi")
	return cmd
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
