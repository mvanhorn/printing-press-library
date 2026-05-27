// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: servizi command group — preserve on regeneration.

package cli

import "github.com/spf13/cobra"

func newServiziCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servizi",
		Short: "Ricerca servizi digitali erogati da enti e UO (portale IPA)",
		Long: `Ricerca servizi digitali pubblicati sul portale IPA.

Usa questo gruppo per trovare URL e schede di servizi online come Albo Pretorio,
pagoPA, SUAP, tributi, pratiche edilizie, concorsi, contravvenzioni e appalti.

Workflow tipico:
  1. servizi tipi              # scopri gli ID delle tipologie ente
  2. servizi ente --tipologia 1 # cerca servizi ente per tipologia
  3. servizi tipi --uo          # scopri gli ID delle categorie UO
  4. servizi uo --categoria 25  # cerca uffici per categoria servizio

Nota: questi dati arrivano dal portale IPA, non dai web service pubblici WS01-WS23.`,
		Example: `  # Albo pretorio del Comune di Bari
  openipa-pp-cli servizi ente --nome-ente "Comune di Bari" --nome-servizio "albo" --json

  # Tipologie disponibili per servizi degli enti
  openipa-pp-cli servizi tipi

  # Categorie disponibili per servizi delle UO
  openipa-pp-cli servizi tipi --uo`,
	}
	cmd.AddCommand(newServiziEnteCmd(flags))
	cmd.AddCommand(newServiziUoCmd(flags))
	cmd.AddCommand(newServiziTipiCmd(flags))
	return cmd
}
