// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: rtd command — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
)

var ipaCodeRe = regexp.MustCompile(`^[a-z]+_[a-z0-9]+$`)

func newRtdCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rtd",
		Short: "Responsabile Transizione Digitale — ricerca per nominativo, ente, area",
	}
	cmd.AddCommand(newRtdCercaCmd(flags))
	cmd.AddCommand(newRtdEmailCmd(flags))
	return cmd
}

func newRtdCercaCmd(flags *rootFlags) *cobra.Command {
	var nominativo, area, ente, codiceEnte, categoria string
	var pagina int
	var tutti bool

	cmd := &cobra.Command{
		Use:   "cerca",
		Short: "Cerca RTD per nominativo, area geografica o ente",
		Example: `  openipa-pp-cli rtd cerca --nominativo "Bianchi"
  openipa-pp-cli rtd cerca --ente "Comune di Roma"
  openipa-pp-cli rtd cerca --area "Sicilia" --tutti`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nominativo == "" && area == "" && ente == "" && codiceEnte == "" && categoria == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --nominativo, --ente, --area, --categoria, --codice-ente")
			}
			if ente != "" && codiceEnte == "" && ipaCodeRe.MatchString(ente) {
				fmt.Fprintf(os.Stderr, "nota: '%s' sembra un codice IPA — uso come --codice-ente\n", ente)
				codiceEnte = ente
				ente = ""
			}
			c := newPortaleClient(flags)

			body := map[string]any{
				"nominativoResponsabile": nilIfEmpty(nominativo),
				"area":                  nilIfEmpty(area),
				"denominazioneEnte":     nilIfEmpty(ente),
				"codEnte":               nilIfEmpty(codiceEnte),
				"categoria":             nilIfEmpty(categoria),
			}

			var items json.RawMessage
			var pag pagResp
			var status int
			var err error

			if tutti {
				items, pag, status, err = fetchAllPages(c, "/api/ou/ricercaRespTD", body, "anagraficaResponsabile")
			} else {
				body["paginazione"] = newPaginazione("anagraficaResponsabile", pagina)
				var raw json.RawMessage
				raw, status, err = c.PostJSON("/api/ou/ricercaRespTD", body)
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
			return portaleOutput(cmd.OutOrStdout(), flags, "/api/ou/ricercaRespTD", "rtd", status, items, pag)
		},
	}

	cmd.Flags().StringVar(&nominativo, "nominativo", "", "Nome/cognome del RTD")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica (es. 'Roma', 'Sicilia') — senza sigla provinciale")
	cmd.Flags().StringVar(&ente, "ente", "", "Denominazione dell'ente")
	cmd.Flags().StringVar(&codiceEnte, "codice-ente", "", "Codice IPA dell'ente")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Numero di pagina (30 risultati per pagina)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine e restituisce risultati completi")
	return cmd
}

func newRtdEmailCmd(flags *rootFlags) *cobra.Command {
	var nominativo, area, ente, codiceEnte, categoria string
	var tutti bool

	cmd := &cobra.Command{
		Use:   "email",
		Short: "RTD con email: cerca il RTD e recupera il contatto dalla UO (due chiamate in automatico)",
		Example: `  openipa-pp-cli rtd email --codice-ente r_sicili
  openipa-pp-cli rtd email --ente "Comune di Palermo"
  openipa-pp-cli rtd email --area "Sicilia" --tutti`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nominativo == "" && area == "" && ente == "" && codiceEnte == "" && categoria == "" && !flags.dryRun {
				return fmt.Errorf("almeno un filtro richiesto: --nominativo, --ente, --area, --categoria, --codice-ente")
			}
			if ente != "" && codiceEnte == "" && ipaCodeRe.MatchString(ente) {
				fmt.Fprintf(os.Stderr, "nota: '%s' sembra un codice IPA — uso come --codice-ente\n", ente)
				codiceEnte = ente
				ente = ""
			}

			// Step 1: cerca RTD via PortaleServices
			portale := newPortaleClient(flags)
			body := map[string]any{
				"nominativoResponsabile": nilIfEmpty(nominativo),
				"area":                  nilIfEmpty(area),
				"denominazioneEnte":     nilIfEmpty(ente),
				"codEnte":               nilIfEmpty(codiceEnte),
				"categoria":             nilIfEmpty(categoria),
			}

			var rtdItems json.RawMessage
			var err error
			if tutti {
				rtdItems, _, _, err = fetchAllPages(portale, "/api/ou/ricercaRespTD", body, "anagraficaResponsabile")
			} else {
				body["paginazione"] = newPaginazione("anagraficaResponsabile", 1)
				var raw json.RawMessage
				raw, _, err = portale.PostJSON("/api/ou/ricercaRespTD", body)
				if err == nil {
					var r *portaleResp
					r, err = parsePortaleResp(raw)
					if err == nil {
						if r.Risposta == nil {
							rtdItems = json.RawMessage(`[]`)
						} else {
							rtdItems = r.Risposta.ListaResponse
						}
					}
				}
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var rtdList []map[string]any
			if err := json.Unmarshal(rtdItems, &rtdList); err != nil || len(rtdList) == 0 {
				return fmt.Errorf("nessun RTD trovato")
			}

			// Step 2: per ogni RTD, recupera email via WS06_OU_CODUNI
			ws, err := flags.newClient()
			if err != nil {
				return err
			}

			type rtdEmail struct {
				Nominativo   string `json:"nominativo"`
				Ente         string `json:"ente"`
				Ufficio      string `json:"ufficio"`
				Email        string `json:"email"`
				PEC          string `json:"pec"`
				CodUniOu     string `json:"cod_uni_ou"`
			}

			results := make([]rtdEmail, 0, len(rtdList))
			for _, item := range rtdList {
				codUni, _ := item["codUniOu"].(string)
				nome, _ := item["nomeResponsabile"].(string)
				denEnte, _ := item["denominazioneEnte"].(string)

				r := rtdEmail{
					Nominativo: nome,
					Ente:       denEnte,
					CodUniOu:   codUni,
				}

				if codUni != "" {
					uoRaw, _, uoErr := ws.Post("/WS06_OU_CODUNI.php", map[string]any{"COD_UNI_OU": codUni})
					if uoErr != nil {
						fmt.Fprintf(os.Stderr, "warning: WS06 failed for %s: %v\n", codUni, uoErr)
					} else {
						var uoWrap struct {
							Data []struct {
								DesOU string `json:"des_ou"`
								Mail1 string `json:"mail1"`
								Mail2 string `json:"mail2"`
							} `json:"data"`
						}
						if json.Unmarshal(uoRaw, &uoWrap) == nil && len(uoWrap.Data) > 0 {
							r.Ufficio = uoWrap.Data[0].DesOU
							r.Email = uoWrap.Data[0].Mail1
							r.PEC = uoWrap.Data[0].Mail2
						}
					}
				}
				results = append(results, r)
			}

			// Output
			w := cmd.OutOrStdout()
			if wantsHumanTable(w, flags) {
				rows := make([]map[string]any, len(results))
				for i, r := range results {
					rows[i] = map[string]any{
						"nominativo": r.Nominativo,
						"ente":       r.Ente,
						"ufficio":    r.Ufficio,
						"email":      r.Email,
						"pec":        r.PEC,
					}
				}
				if err := printAutoTable(w, rows); err == nil {
					return nil
				}
			}
			envelope := map[string]any{
				"action":   "compound",
				"resource": "rtd/email",
				"success":  true,
				"data":     results,
			}
			envelopeJSON, _ := json.Marshal(envelope)
			return printOutput(w, json.RawMessage(envelopeJSON), true)
		},
	}

	cmd.Flags().StringVar(&nominativo, "nominativo", "", "Nome/cognome del RTD")
	cmd.Flags().StringVar(&area, "area", "", "Comune o area geografica — senza sigla provinciale")
	cmd.Flags().StringVar(&ente, "ente", "", "Denominazione dell'ente")
	cmd.Flags().StringVar(&codiceEnte, "codice-ente", "", "Codice IPA dell'ente")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Codice categoria IPA")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Scarica tutte le pagine (attenzione: una chiamata WS06 per ogni RTD trovato)")
	return cmd
}
