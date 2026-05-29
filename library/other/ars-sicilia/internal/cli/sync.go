// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB        string
		flagMaxPages  int
		flagFull      bool
		flagResources []string
		flagLegisle   string
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
				out := map[string]any{
					"dry_run":    true,
					"resources":  flagResources,
					"max_pages":  maxPages,
					"legisl":     flagLegisle,
					"would_sync": "all 12 ARS archives",
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return runSyncAll(cmd, flags, flagDB, maxPages, flagResources, flagLegisle)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite (default: ~/.local/share/ars-sicilia-pp-cli/store.db).")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 5, "Numero massimo di pagine per archivio (0 = tutte).")
	cmd.Flags().BoolVar(&flagFull, "full", false, "Scarica tutte le pagine disponibili (equivale a --max-pages 0).")
	cmd.Flags().StringSliceVar(&flagResources, "resources", nil, "Archivi da sincronizzare (default: tutti). Es: --resources ddl,leggi")
	cmd.Flags().StringVar(&flagLegisle, "legisl", "", "Filtra per legislatura (es. 18).")

	cmd.AddCommand(newNovelSyncStaleCmd(flags))
	return cmd
}
