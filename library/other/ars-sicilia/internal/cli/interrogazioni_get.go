// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInterrogazioniGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <legisl> <numero>",
		Short:   "Scarica un singolo documento da interrogazioni.",
		Example: "  ars-sicilia-pp-cli interrogazioni get 18 1500 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"pp:endpoint":   "interrogazioni.get",
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
			return runGet(cmd, flags, "interrogazioni", legisl, numero)
		},
	}
	return cmd
}
