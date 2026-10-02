// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelStationsCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "stations"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.query, "query", "", "English/Japanese substring or exact station ID to search")
	cmd.Flags().IntVar(&o.limit, "limit", 20, "Maximum returned stations; allowed1–50")
	cmd.Flags().BoolVar(&o.detail, "detail", false, "Include detailed records or fetch public product-document links")
	return cmd
}
