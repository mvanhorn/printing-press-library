// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLeggiGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <legisl> <numero>",
		Short:   "Scarica una singola legge regionale.",
		Example: "  ars-sicilia-pp-cli leggi get 18 1 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"pp:endpoint":   "leggi.get",
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
			return runGet(cmd, flags, "leggi", legisl, numero)
		},
	}
	return cmd
}
