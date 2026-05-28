// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: servizi uo command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newServiziUoCmd(flags *rootFlags) *cobra.Command {
	var categoria int
	var nomeUo, area string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "uo",
		Short: "Cerca unità organizzative per categoria di servizio erogato",
		Long: `Cerca unità organizzative registrate nel portale IPA in base al servizio erogato.

È utile quando cerchi l'ufficio competente per un servizio, ad esempio
protocollo, anagrafe, tributi, URP, edilizia privata, gare e appalti.

Usa "servizi tipi --uo" per scoprire gli ID da passare a --categoria.`,
		Example: `  # Cerca UO che citano protocollo
  openipa-pp-cli servizi uo --nome-uo "protocollo" --json

  # Cerca UO anagrafiche: categoria 4 = Anagrafico
  openipa-pp-cli servizi uo --categoria 4 --json

  # Cerca tutte le UO in un'area
  openipa-pp-cli servizi uo --area "Milano" --tutti --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if categoria == 0 && nomeUo == "" && area == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --categoria, --nome-uo, --area")
			}
			c := newPortaleClient(flags)

			var categoriaVal any
			if categoria > 0 {
				categoriaVal = categoria
			}

			body := map[string]any{
				"categoriaServizio": categoriaVal,
				"descrizione":       nilIfEmpty(nomeUo),
				"area":              nilIfEmpty(area),
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/serviziufficio/ricercaServiziUO", body, "denominazione")
			} else {
				body["paginazione"] = newPaginazione("denominazione", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/serviziufficio/ricercaServiziUO", body)
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
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/serviziufficio/ricercaServiziUO", "servizi/uo", status, items, pag)
		},
	}

	cmd.Flags().IntVar(&categoria, "categoria", 0, "ID categoria servizio UO (usa 'servizi tipi --uo' per la lista; es. 4=Anagrafico, 25=Protocollo)")
	cmd.Flags().StringVar(&nomeUo, "nome-uo", "", "Parte del nome della UO o della descrizione servizio (es. protocollo, anagrafe)")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine")
	return cmd
}
