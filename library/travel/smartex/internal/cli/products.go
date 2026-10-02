// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelProductsCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "products"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.from, "from", "", "Origin station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.to, "to", "", "Destination station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.date, "date", "", "Boarding date YYYY-MM-DD in JST; fare defaults to today")
	cmd.Flags().IntVar(&o.adults, "adults", 1, "Adults; adult plus child-fare passenger count must total1–6")
	cmd.Flags().IntVar(&o.children, "children", 0, "Child-fare passengers; child prices remain unquoted")
	cmd.Flags().StringVar(&o.now, "now", "", "RFC3339 reference instant with offset; default current JST")
	cmd.Flags().StringVar(&o.train, "train", "", "Optional train category: nozomi, hikari, kodama, mizuho, sakura, tsubame")
	cmd.Flags().BoolVar(&o.detail, "detail", false, "Include detailed records or fetch public product-document links")
	cmd.Flags().StringVar(&o.class, "class", "", "Optional class to check known product restrictions")
	return cmd
}
