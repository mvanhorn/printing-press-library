// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelRouteCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "route"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.from, "from", "", "Origin station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.to, "to", "", "Destination station ID, English name or Japanese name")
	cmd.Flags().BoolVar(&o.detail, "detail", false, "Include detailed records or fetch public product-document links")
	return cmd
}
