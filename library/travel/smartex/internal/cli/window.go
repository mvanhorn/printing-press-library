// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelWindowCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "window"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.date, "date", "", "Boarding date YYYY-MM-DD in JST; fare defaults to today")
	cmd.Flags().IntVar(&o.adults, "adults", 1, "Adults; adult plus child-fare passenger count must total1–6")
	cmd.Flags().IntVar(&o.children, "children", 0, "Child-fare passengers; child prices remain unquoted")
	cmd.Flags().StringVar(&o.now, "now", "", "RFC3339 reference instant with offset; default current JST")
	cmd.Flags().StringVar(&o.product, "product", "smart-ex", "Product ID; run products for current IDs and rules")
	cmd.Flags().StringVar(&o.departure, "departure", "", "Timetable departure HH:MM JST for basic four-minute cutoff")
	cmd.Flags().BoolVar(&o.oversized, "oversized-baggage", false, "Require oversized-area seating; one-month known sales boundary")
	cmd.Flags().StringVar(&o.class, "class", "reserved", "reserved|unreserved|green; oversized parties are limited to five ordinary or four Green")
	return cmd
}
