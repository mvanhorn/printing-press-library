// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB        string
		flagMaxPages  int
		flagFull      bool
		flagResources []string
		flagLegisle   string
		flagDeep      bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sincronizza il portale ARS nel database locale SQLite",
		Long: `Sincronizza tutti i 12 archivi ARS nel database SQLite locale.
Successivamente, i comandi analytics, ddl drift e sync stale useranno i dati locali.`,
		Example: `  # Sincronizza tutti gli archivi (5 pagine ciascuno per default)
  ars-sicilia-pp-cli sync

  # Solo DDL, tutte le pagine disponibili
  ars-sicilia-pp-cli sync --resources ddl --max-pages 0

  # Solo legislatura 18, archivi selezionati
  ars-sicilia-pp-cli sync --resources ddl,leggi,interrogazioni --legisl 18`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			maxPages := flagMaxPages
			if flagFull {
				maxPages = 0
			}
			if dryRunOK(flags) || cliIsVerify() {
				// `would_sync` era la stringa fissa "all 12 ARS archives", che
				// con --resources contraddiceva il campo `resources` stampato
				// una riga sopra: `--resources ddl` annunciava comunque dodici
				// archivi. Ora l'elenco esce da filterArchives, la stessa che
				// runSyncAll usa per decidere cosa sincronizzare davvero.
				archivi := filterArchives(icaro.All, flagResources)
				slugs := make([]string, 0, len(archivi))
				for _, a := range archivi {
					slugs = append(slugs, a.Slug)
				}
				out := map[string]any{
					"dry_run":    true,
					"resources":  flagResources,
					"max_pages":  maxPages,
					"legisl":     flagLegisle,
					"deep":       flagDeep,
					"would_sync": slugs,
				}
				if len(slugs) == 0 {
					// Stesso esito del percorso vivo, che qui fallisce invece
					// di sincronizzare nulla in silenzio.
					out["nota"] = fmt.Sprintf("nessun archivio corrisponde al filtro %v: la sincronizzazione fallirebbe", flagResources)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return runSyncAll(cmd, flags, flagDB, maxPages, flagResources, flagLegisle, flagDeep)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite (default: ~/.local/share/ars-sicilia-pp-cli/data.db).")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 5, "Numero massimo di pagine per archivio (0 = tutte).")
	cmd.Flags().BoolVar(&flagFull, "full", false, "Scarica tutte le pagine disponibili (equivale a --max-pages 0).")
	cmd.Flags().StringSliceVar(&flagResources, "resources", nil, "Archivi da sincronizzare (default: tutti). Es: --resources ddl,leggi")
	cmd.Flags().StringVar(&flagLegisle, "legisl", "", "Filtra per legislatura (es. 18).")
	cmd.Flags().BoolVar(&flagDeep, "deep", false, "Per i ddl, scarica anche la scheda di dettaglio di ogni record per estrarre firmatari e stato dell'iter (sblocca `analytics --group-by cofirmatari` e `ddl drift`). Molto più lento: ~1 richiesta extra per ddl.")

	cmd.AddCommand(newNovelSyncStaleCmd(flags))
	cmd.AddCommand(newNovelSyncCoverageCmd(flags))
	return cmd
}
