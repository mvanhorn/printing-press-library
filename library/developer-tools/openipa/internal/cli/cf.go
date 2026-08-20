// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// cfResult aggregates all IPA channels for a single codice fiscale.
type cfResult struct {
	CF          string           `json:"cf"`
	SFE         []map[string]any `json:"sfe,omitempty"`
	NSO         []map[string]any `json:"nso,omitempty"`
	DomDigitale []map[string]any `json:"domicilio_digitale,omitempty"`
	SFEError    string           `json:"sfe_error,omitempty"`
	NSOError    string           `json:"nso_error,omitempty"`
	DomError    string           `json:"dom_error,omitempty"`
	SFEStatus   string           `json:"sfe_status"`
	NSOStatus   string           `json:"nso_status"`
	DomStatus   string           `json:"dom_status"`
}

func newCfCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cf <codice-fiscale>",
		Short: "Tutti i canali IPA di un ente per codice fiscale (SFE + NSO + domicilio digitale)",
		Long: `Dato il codice fiscale di un ente PA, interroga in parallelo:
  - WS01_SFE_CF: uffici destinatari fattura elettronica su SDI
  - WS14_NSO_CF: nodi smistamento ordini elettronici
  - WS23_DOM_DIG_CF: domicilio digitale attivo

Produce una checklist pass/fail per ogni canale. Utilissimo per verificare
la compliance PA prima di emettere fatture o ordini.`,
		Example: strings.Trim(`
  openipa-pp-cli cf 97735020584
  openipa-pp-cli cf 97735020584 --json
  openipa-pp-cli cf 80012000826 --json --select cf,sfe_status,nso_status,dom_status`, "\n"),
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

			cf := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := cfResult{CF: cf}
			var wg sync.WaitGroup
			var mu sync.Mutex

			// WS01_SFE_CF — uffici fatturazione elettronica
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS01SFECFServices/api/WS01_SFE_CF", map[string]any{"CF": cf})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					result.SFEError = callErr.Error()
					result.SFEStatus = "error"
					return
				}
				items := ipaExtractItems(raw)
				result.SFE = items
				if len(items) > 0 {
					result.SFEStatus = "attivo"
				} else {
					result.SFEStatus = "non trovato"
				}
			}()

			// WS14_NSO_CF — nodi smistamento ordini
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS14NSOCFServices/api/WS14_NSO_CF", map[string]any{"CF": cf})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					result.NSOError = callErr.Error()
					result.NSOStatus = "error"
					return
				}
				items := ipaExtractItems(raw)
				result.NSO = items
				if len(items) > 0 {
					result.NSOStatus = "attivo"
				} else {
					result.NSOStatus = "non trovato"
				}
			}()

			// WS23_DOM_DIG_CF — domicilio digitale (may return HTTP 500 on IPA server)
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS23DOMDIGCFServices/api/WS23_DOM_DIG_CF", map[string]any{"CF": cf})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					result.DomError = callErr.Error()
					result.DomStatus = "error"
					return
				}
				items := ipaExtractItems(raw)
				result.DomDigitale = items
				if len(items) > 0 {
					result.DomStatus = "presente"
				} else {
					result.DomStatus = "non trovato"
				}
			}()

			wg.Wait()

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Human-readable checklist
			w := cmd.OutOrStdout()
			ok := "✓"
			fail := "✗"

			fmt.Fprintf(w, "CF: %s\n\n", cf)

			// SFE
			sfeIcon := fail
			if result.SFEStatus == "attivo" {
				sfeIcon = ok
			}
			fmt.Fprintf(w, "%s Fatturazione Elettronica (SFE): %s\n", sfeIcon, result.SFEStatus)
			for _, ente := range result.SFE {
				des, _ := ente["des_amm"].(string)
				cod, _ := ente["cod_amm"].(string)
				fmt.Fprintf(w, "   Ente: %s (%s)\n", des, cod)
				if ous, ok := ente["OU"].([]interface{}); ok {
					for _, ouRaw := range ous {
						if ou, ok := ouRaw.(map[string]any); ok {
							desOU, _ := ou["des_ou"].(string)
							codUni, _ := ou["cod_uni_ou"].(string)
							stato, _ := ou["stato_canale"].(string)
							fmt.Fprintf(w, "   → %s (cod_uni_ou: %s, stato: %s)\n", desOU, codUni, stato)
						}
					}
				}
			}

			// NSO
			nsoIcon := fail
			if result.NSOStatus == "attivo" {
				nsoIcon = ok
			}
			fmt.Fprintf(w, "%s Smistamento Ordini (NSO): %s\n", nsoIcon, result.NSOStatus)
			for _, ente := range result.NSO {
				des, _ := ente["des_amm"].(string)
				cod, _ := ente["cod_amm"].(string)
				fmt.Fprintf(w, "   Ente: %s (%s)\n", des, cod)
				if ous, ok := ente["OU"].([]interface{}); ok {
					for _, ouRaw := range ous {
						if ou, ok := ouRaw.(map[string]any); ok {
							desOU, _ := ou["des_ou"].(string)
							codUni, _ := ou["cod_uni_ou"].(string)
							stato, _ := ou["stato_canale"].(string)
							fmt.Fprintf(w, "   → %s (cod_uni_ou: %s, stato: %s)\n", desOU, codUni, stato)
						}
					}
				}
			}

			// Domicilio digitale
			domIcon := fail
			if result.DomStatus == "presente" {
				domIcon = ok
			}
			fmt.Fprintf(w, "%s Domicilio Digitale: %s\n", domIcon, result.DomStatus)
			for _, dom := range result.DomDigitale {
				addr, _ := dom["domicilio_digitale"].(string)
				tipo, _ := dom["tipo"].(string)
				fmt.Fprintf(w, "   → %s (tipo: %s)\n", addr, tipo)
			}

			return nil
		},
	}
	return cmd
}
