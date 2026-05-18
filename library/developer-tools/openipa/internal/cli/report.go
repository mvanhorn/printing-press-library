//go:build ignore

// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report e analisi aggregati su dati IPA locali",
		Long:  "Comandi di analisi aggregata sulle entità IPA sincronizzate localmente.",
	}
	cmd.AddCommand(newReportSfeMancante(flags))
	return cmd
}

func newReportSfeMancante(flags *rootFlags) *cobra.Command {
	var regione, categoria string

	cmd := &cobra.Command{
		Use:   "sfe-mancante",
		Short: "Lista enti senza canale SFE attivo (audit compliance fatturazione)",
		Long: `Elenca tutti gli enti PA non abilitati alla fatturazione elettronica
su SDI, filtrabile per regione e categoria — per audit compliance.

NOTA: Questa funzionalità non è ancora disponibile. Sarà implementata
in una versione futura con supporto SQLite locale.`,
		Example: strings.Trim(`
  openipa-pp-cli report sfe-mancante
  openipa-pp-cli report sfe-mancante --regione Sicilia --json`, "\n"),
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
	cmd.Flags().StringVar(&categoria, "categoria", "", "Filtra per categoria ente")
	return cmd
}
