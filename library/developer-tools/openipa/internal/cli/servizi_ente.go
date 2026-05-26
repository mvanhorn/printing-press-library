// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: servizi ente command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newServiziEnteCmd(flags *rootFlags) *cobra.Command {
	var tipologia int
	var nomeServizio, nomeEnte, area, categoria, ateco string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "ente",
		Short: "Cerca enti per servizio digitale erogato",
		Long: `Cerca servizi digitali associati agli enti nel portale IPA.

Restituisce, quando disponibile, l'URL del servizio nel campo uri. È utile per
recuperare Albo Pretorio online, pagoPA, SUAP, tributi, pratiche edilizie,
concorsi, contravvenzioni, appalti e altri servizi pubblicati dall'ente.

Usa "servizi tipi" per scoprire gli ID da passare a --tipologia. Ad esempio,
la tipologia 1 è "Accesso agli atti" e comprende spesso l'Albo Pretorio.

Attenzione: --nome-ente è una ricerca testuale ampia. "Comune di Bari" può
restituire anche Bariano, Baricella o Barisardo; filtra il JSON per
.denominazioneEnte quando serve una corrispondenza esatta.`,
		Example: `  # Albo pretorio del Comune di Bari
  openipa-pp-cli servizi ente --nome-ente "Comune di Bari" --nome-servizio "albo" --json

  # Solo l'URL dell'Albo Pretorio, filtrando la denominazione esatta
  openipa-pp-cli servizi ente --nome-ente "Comune di Bari" --nome-servizio "albo" --json | \
    jq -r '.data[] | select(.denominazioneEnte == "Comune di Bari") | .uri'

  # Tutti gli Albi Pretori / accesso agli atti nell'area Bari
  openipa-pp-cli servizi ente --area "Bari" --tipologia 1 --json

  # Cerca servizi pagoPA
  openipa-pp-cli servizi ente --nome-servizio "pagopa" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tipologia == 0 && nomeServizio == "" && nomeEnte == "" && area == "" && categoria == "" && ateco == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --tipologia, --nome-servizio, --nome-ente, --area, --categoria, --ateco")
			}
			c := newPortaleClient(flags)

			var tipologiaVal any
			if tipologia > 0 {
				tipologiaVal = tipologia
			}

			body := map[string]any{
				"tipologiaServizio":     tipologiaVal,
				"denominazioneServizio": nilIfEmpty(nomeServizio),
				"denominazioneEnte":     nilIfEmpty(nomeEnte),
				"area":                  nilIfEmpty(area),
				"categoria":             nilIfEmpty(categoria),
				"attivitaAteco":         nilIfEmpty(ateco),
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/servizidigitali/ricerca", body, "denominazione")
			} else {
				body["paginazione"] = newPaginazione("denominazione", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/servizidigitali/ricerca", body)
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
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/servizidigitali/ricerca", "servizi/ente", status, items, pag)
		},
	}

	cmd.Flags().IntVar(&tipologia, "tipologia", 0, "ID tipologia servizio (usa 'servizi tipi' per la lista; 1=Accesso agli atti/Albo Pretorio)")
	cmd.Flags().StringVar(&nomeServizio, "nome-servizio", "", "Parte del nome del servizio (es. albo, pagopa, SUAP)")
	cmd.Flags().StringVar(&nomeEnte, "nome-ente", "", "Parte del nome dell'ente; ricerca testuale ampia, non exact match")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().StringVar(&ateco, "ateco", "", "Codice attività ATECO")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine")
	return cmd
}
