// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/cliutil"

	"github.com/spf13/cobra"
)

type batchSFEResult struct {
	CF     string           `json:"cf"`
	Enti   []map[string]any `json:"enti,omitempty"`
	Error  string           `json:"error,omitempty"`
	Status string           `json:"status"`
}

func newFatturazioneBatchCmd(flags *rootFlags) *cobra.Command {
	var concurrency int

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Lookup CF → codici destinatario SDI in batch da stdin",
		Long: `Legge codici fiscali da stdin (uno per riga, righe vuote ignorate),
chiama WS01_SFE_CF in parallelo per ciascuno, e restituisce NDJSON con
CF + cod_uni_ou + stato_canale per ogni ufficio trovato.

Utile per pipeline di fatturazione dove si devono trovare i destinatari
SDI per una lista di enti PA in un solo passaggio.`,
		Example: strings.Trim(`
  echo "97735020584" | openipa-pp-cli fatturazione batch
  cat lista_cf.txt | openipa-pp-cli fatturazione batch --concurrency 5
  cat lista_cf.txt | openipa-pp-cli fatturazione batch --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var cfs []string
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					cfs = append(cfs, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("lettura stdin: %w", err)
			}
			if len(cfs) == 0 {
				return fmt.Errorf("nessun CF letto da stdin (uno per riga)")
			}

			results, fanoutErrs := cliutil.FanoutRun(
				cmd.Context(),
				cfs,
				func(cf string) string { return cf },
				func(_ context.Context, cfVal string) (batchSFEResult, error) {
					res := batchSFEResult{CF: cfVal}
					raw, _, callErr := c.Post("/WS01_SFE_CF.php", map[string]any{"CF": cfVal})
					if callErr != nil {
						res.Error = callErr.Error()
						res.Status = "error"
						return res, nil
					}
					var env struct {
						Data json.RawMessage `json:"data"`
					}
					var items []map[string]any
					if json.Unmarshal(raw, &env) == nil && env.Data != nil {
						if parseErr := json.Unmarshal(env.Data, &items); parseErr != nil {
							res.Error = "parse error: " + parseErr.Error()
							res.Status = "error"
							return res, nil
						}
					} else {
						var appErr struct {
							Errore      int    `json:"errore"`
							Descrizione string `json:"descrizioneErrore"`
						}
						if json.Unmarshal(raw, &appErr) == nil && appErr.Errore != 0 {
							res.Error = appErr.Descrizione
							res.Status = "error"
							return res, nil
						}
						json.Unmarshal(raw, &items)
					}
					res.Enti = items
					if len(items) > 0 {
						res.Status = "trovato"
					} else {
						res.Status = "non trovato"
					}
					return res, nil
				},
				cliutil.WithConcurrency(concurrency),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), fanoutErrs)

			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, r := range results {
				if err := enc.Encode(r.Value); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Numero di richieste parallele (default 3)")
	return cmd
}
