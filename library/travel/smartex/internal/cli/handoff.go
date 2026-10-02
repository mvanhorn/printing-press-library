// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelHandoffCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "handoff"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.from, "from", "", "Origin station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.to, "to", "", "Destination station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.date, "date", "", "Boarding date YYYY-MM-DD in JST; fare defaults to today")
	cmd.Flags().StringVar(&o.class, "class", "reserved", "Seat class: reserved, unreserved or green; fare also supports all")
	cmd.Flags().IntVar(&o.adults, "adults", 1, "Adults; adult plus child-fare passenger count must total1–6")
	cmd.Flags().IntVar(&o.children, "children", 0, "Child-fare passengers; child prices remain unquoted")
	return cmd
}
