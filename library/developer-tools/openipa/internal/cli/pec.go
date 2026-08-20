// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(WS-endpoint-migration): new command group for IPA PEC web services (WS20/WS21/WS22).

package cli

import (
	"github.com/spf13/cobra"
)

func newPecCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pec",
		Short: "Indirizzi PEC degli enti IPA",
		Long: `Comandi per recuperare indirizzi PEC degli enti dall'Indice PA.

  pec ente <cod_amm>    PEC attive di un ente (WS20)
  pec storico <cod_amm> Storico PEC di un ente (WS21)
  pec cerca <indirizzo> Storia di un indirizzo PEC (WS22)`,
	}

	cmd.AddCommand(newPecEnteCmd(flags))
	cmd.AddCommand(newPecStoricoCmd(flags))
	cmd.AddCommand(newPecCercaCmd(flags))
	return cmd
}
