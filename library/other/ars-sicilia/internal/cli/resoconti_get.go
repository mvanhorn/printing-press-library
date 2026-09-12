// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResocontiGetCmd(flags *rootFlags) *cobra.Command {
	var conTesto bool
	cmd := &cobra.Command{
		Use:     "get <legisl> <numero>",
		Short:   "Scarica un singolo documento da resoconti.",
		Example: "  ars-sicilia-pp-cli resoconti get 17 208 --con-testo --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"pp:endpoint":   "resoconti.get",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			legisl, err := atoiArg(args[0], "legisl")
			if err != nil {
				return err
			}
			numero, err := atoiArg(args[1], "numero")
			if err != nil {
				return err
			}
			return runGetOpts(cmd, flags, "resoconti", legisl, numero, nil, getOpts{conTesto: conTesto})
		},
	}
	// Fuori per default: il testo di una seduta va dalle 8.000 alle 95.000
	// battute (misurato sulle sedute dalla XIII alla XVIII), e chi apre la
	// scheda per sapere chi ha parlato o per prendere `pdf_url` non le vuole
	// addosso. Chiedendolo non costa comunque una richiesta in più: la pagina
	// che lo contiene `get` la scarica già.
	cmd.Flags().BoolVar(&conTesto, "con-testo", false, "Aggiunge il campo `testo` con la versione testuale della seduta, quando il portale la pubblica.")
	return cmd
}
