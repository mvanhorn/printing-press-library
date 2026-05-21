package cli

import "github.com/spf13/cobra"

// newKTEnergyCmd hosts the energy balance command.
func newKTEnergyCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:   "energy",
		Short: "Energy in/out analytics across diary windows",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	parent.AddCommand(newKTEnergyBalanceCmd(flags))
	return parent
}
