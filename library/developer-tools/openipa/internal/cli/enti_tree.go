// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// enteTree is the hierarchical view of an ente with its AOO and UO.
type enteTree struct {
	CodAmm   string        `json:"cod_amm"`
	DesAmm   string        `json:"des_amm"`
	Regione  string        `json:"regione,omitempty"`
	Comune   string        `json:"comune,omitempty"`
	SitoWeb  string        `json:"sito_istituzionale,omitempty"`
	CF       string        `json:"cf,omitempty"`
	AOO      []aooTreeNode `json:"aoo,omitempty"`
	UO       []uoTreeNode  `json:"uo,omitempty"`
	AOOCount int           `json:"aoo_count"`
	UOCount  int           `json:"uo_count"`
}

type aooTreeNode struct {
	CodAOO string `json:"cod_aoo"`
	DesAOO string `json:"des_aoo"`
	Mail1  string `json:"mail1,omitempty"`
	Comune string `json:"comune,omitempty"`
}

type uoTreeNode struct {
	CodUniOU string `json:"cod_uni_ou"`
	DesOU    string `json:"des_ou"`
	CodAOO   string `json:"cod_aoo,omitempty"`
	Mail1    string `json:"mail1,omitempty"`
	Comune   string `json:"comune,omitempty"`
}

func newEntiTreeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree <cod-amm>",
		Short: "Vista gerarchica di un ente: ente → AOO[N] → UO[M]",
		Long: `Chiama in parallelo WS05_AMM + WS02_AOO + WS03_OU e costruisce
la struttura organizzativa completa di un ente IPA in un unico output.`,
		Example: strings.Trim(`
  openipa-pp-cli enti tree agid
  openipa-pp-cli enti tree r_sicili --json
  openipa-pp-cli enti tree agid --json --select cod_amm,des_amm,aoo_count,uo_count`, "\n"),
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

			codAmm := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			tree := enteTree{CodAmm: codAmm}
			var wg sync.WaitGroup
			var mu sync.Mutex
			var errList []string

			// Use shared IPA extraction helpers from ipa_helpers.go

			// WS05_AMM — dati base ente
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS05AMMServices/api/WS05_AMM", map[string]any{"COD_AMM": codAmm})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					errList = append(errList, "WS05_AMM: "+callErr.Error())
					return
				}
				item := ipaExtractSingle(raw)
				if item == nil {
					items := ipaExtractItems(raw)
					if len(items) > 0 {
						item = items[0]
					}
				}
				if item != nil {
					if v, ok := item["des_amm"].(string); ok {
						tree.DesAmm = v
					}
					if v, ok := item["regione"].(string); ok {
						tree.Regione = v
					}
					if v, ok := item["comune"].(string); ok {
						tree.Comune = v
					}
					if v, ok := item["sito_istituzionale"].(string); ok {
						tree.SitoWeb = v
					}
					if v, ok := item["cf"].(string); ok {
						tree.CF = v
					}
				}
			}()

			// WS02_AOO — aree organizzative omogenee
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS02AOOServices/api/WS02_AOO", map[string]any{"COD_AMM": codAmm})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					errList = append(errList, "WS02_AOO: "+callErr.Error())
					return
				}
				items := ipaExtractItems(raw)
				for _, item := range items {
					node := aooTreeNode{}
					if v, ok := item["cod_aoo"].(string); ok {
						node.CodAOO = v
					}
					if v, ok := item["des_aoo"].(string); ok {
						node.DesAOO = v
					}
					if v, ok := item["mail1"].(string); ok {
						node.Mail1 = v
					}
					if v, ok := item["comune"].(string); ok {
						node.Comune = v
					}
					tree.AOO = append(tree.AOO, node)
				}
				tree.AOOCount = len(items)
			}()

			// WS03_OU — unità organizzative
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, _, callErr := c.Post("/ws/WS03OUServices/api/WS03_OU", map[string]any{"COD_AMM": codAmm})
				mu.Lock()
				defer mu.Unlock()
				if callErr != nil {
					errList = append(errList, "WS03_OU: "+callErr.Error())
					return
				}
				items := ipaExtractItems(raw)
				for _, item := range items {
					node := uoTreeNode{}
					if v, ok := item["cod_uni_ou"].(string); ok {
						node.CodUniOU = v
					}
					if v, ok := item["des_ou"].(string); ok {
						node.DesOU = v
					}
					if v, ok := item["cod_aoo"].(string); ok {
						node.CodAOO = v
					}
					if v, ok := item["mail1"].(string); ok {
						node.Mail1 = v
					}
					if v, ok := item["comune"].(string); ok {
						node.Comune = v
					}
					tree.UO = append(tree.UO, node)
				}
				tree.UOCount = len(items)
			}()

			wg.Wait()

			if len(errList) > 0 {
				return fmt.Errorf("errori nel recupero dati: %s", strings.Join(errList, "; "))
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(tree)
			}

			// Human-readable tree
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "📋 %s (%s)\n", tree.DesAmm, tree.CodAmm)
			if tree.Regione != "" {
				fmt.Fprintf(w, "   %s, %s\n", tree.Comune, tree.Regione)
			}
			if tree.CF != "" {
				fmt.Fprintf(w, "   CF: %s\n", tree.CF)
			}
			fmt.Fprintf(w, "   AOO: %d | UO: %d\n\n", tree.AOOCount, tree.UOCount)

			for _, aoo := range tree.AOO {
				fmt.Fprintf(w, "  ├─ AOO %s: %s\n", aoo.CodAOO, aoo.DesAOO)
				if aoo.Mail1 != "" {
					fmt.Fprintf(w, "  │   PEC: %s\n", aoo.Mail1)
				}
				// Show UOs belonging to this AOO
				for _, uo := range tree.UO {
					if uo.CodAOO == aoo.CodAOO {
						fmt.Fprintf(w, "  │  └─ UO %s: %s\n", uo.CodUniOU, uo.DesOU)
					}
				}
			}

			// Show UOs not belonging to any AOO (cod_aoo empty)
			for _, uo := range tree.UO {
				if uo.CodAOO == "" {
					fmt.Fprintf(w, "  └─ UO %s: %s\n", uo.CodUniOU, uo.DesOU)
				}
			}

			return nil
		},
	}
	return cmd
}
