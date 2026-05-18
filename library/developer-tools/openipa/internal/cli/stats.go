//go:build ignore

// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var regione, categoria string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Statistiche aggregati enti IPA per regione/categoria (richiede sync locale)",
		Long: `Aggrega enti, AOO, UO e percentuale con canali SFE/NSO attivi per
regione o categoria — tutto da SQLite locale senza API call.

NOTA: Questa funzionalità non è ancora disponibile. Sarà implementata
in una versione futura con supporto SQLite locale.`,
		Example: strings.Trim(`
  openipa-pp-cli stats --regione Lazio
  openipa-pp-cli stats --regione Sicilia --categoria "Comuni e loro Consorzi" --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return fmt.Errorf("non ancora disponibile: richiede sincronizzazione locale SQLite (in sviluppo)")
		},
	}
	cmd.Flags().StringVar(&regione, "regione", "", "Filtra per regione (es. 'Lazio', 'Sicilia')")
	cmd.Flags().StringVar(&categoria, "categoria", "", "Filtra per categoria ente (es. 'Comuni e loro Consorzi')")
	return cmd
}
