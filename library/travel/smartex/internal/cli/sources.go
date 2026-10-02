// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelSourcesCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "sources"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.sourceID, "source", "", "Exact source ID; default bounded catalog")
	cmd.Flags().BoolVar(&o.check, "check", false, "Make live public page HTTP reachability checks")
	cmd.Flags().IntVar(&o.limit, "limit", 3, "Maximum catalog entries or live checks; allowed1–50")
	cmd.Flags().BoolVar(&o.detail, "detail", false, "Include detailed records or fetch public product-document links")
	return cmd
}
