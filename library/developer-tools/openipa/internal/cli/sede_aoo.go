// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: sede aoo command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSedeAooCmd(flags *rootFlags) *cobra.Command {
	var nome, cf, area, categoria, codiceEnte, codiceAoo string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "aoo",
		Short: "Cerca AOO per sede: nome, area geografica, ente",
		Example: `  openipa-pp-cli sede aoo --nome "protocollo"
  openipa-pp-cli sede aoo --codice-ente c_g273
  openipa-pp-cli sede aoo --nome "Prefettura - UTG - PALERMO"
  openipa-pp-cli sede aoo --nome "prefettura" --tutti`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nome == "" && cf == "" && area == "" && categoria == "" && codiceEnte == "" && codiceAoo == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --nome, --cf, --area, --categoria, --codice-ente, --codice-aoo")
			}
			c := newPortaleClient(flags)

			body := map[string]any{
				"desAoo":            nilIfEmpty(nome),
				"codiceFiscale":     nilIfEmpty(cf),
				"area":              nilIfEmpty(area),
				"codiceCategoria":   nilIfEmpty(categoria),
				"denominazioneEnte": nil,
				"codEnte":           nilIfEmpty(codiceEnte),
				"codUniAoo":         nilIfEmpty(codiceAoo),
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/aoo", body, "codAoo")
			} else {
				body["paginazione"] = newPaginazione("codAoo", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/aoo", body)
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
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/aoo", "sede/aoo", status, items, pag)
		},
	}

	cmd.Flags().StringVar(&nome, "nome", "", "Parte del nome dell'AOO")
	cmd.Flags().StringVar(&cf, "cf", "", "Codice fiscale dell'ente")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica (es. 'Palermo', 'Roma') — senza sigla provinciale")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().StringVar(&codiceEnte, "codice-ente", "", "Codice IPA dell'ente")
	cmd.Flags().StringVar(&codiceAoo, "codice-aoo", "", "Codice univoco AOO")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine e restituisce risultati completi")
	return cmd
}
